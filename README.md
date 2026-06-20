# LP Vol Analyzer

A crypto liquidity-pool analyzer that does two things:

1. **Tracks one LP position and its hedge** — reads an Aerodrome (Base)
   position from chain, prices it, runs the fee-vs-volatility model over its
   pool, and reports the Binance perpetual **short** needed to hedge the
   position's directional exposure (target vs. current short, drift, PnL).
2. **Auto-scans the most profitable pools across chains** — every
   `SCAN_INTERVAL` it pulls the most active pools across multiple L1s and L2s
   and ranks them by how much their fee yield beats the assets' **price
   volatility**, with a focus on Aerodrome, Velodrome, PancakeSwap and Uniswap.

It ships as a single Go binary: a background scanner + position tracker + JSON
API + a zero-build web dashboard.

![dashboard](https://img.shields.io/badge/stack-Go%20%2B%20vanilla%20JS-blue)

## The model

A liquidity provider in a constant-product AMM continuously loses value to
arbitrageurs as the price moves — this is **loss-versus-rebalancing (LVR)**.
For an asset with annualised volatility `σ`, the LVR cost per year, as a
fraction of pooled value, is (Milionis, Moallemi, Roughgarden & Zhang, 2022):

```
LVR_cost = σ² / 8
```

An LP only profits when fee income outruns that loss. We compute fee income as
an APR on TVL:

```
feeAPR = volume_24h × feeTier / TVL × 365
```

Setting fee income equal to the LVR cost and solving for `σ` gives the
**fee-implied volatility** — the volatility level the pool's fees are effectively
"pricing in":

```
σ_fees = √(8 × feeAPR)
```

We then compare three volatilities:

| Volatility | Source |
|---|---|
| **Fee-implied σ** | `√(8·feeAPR)` — what the fees pay for |
| **Realized σ** | log-return stdev of 7d hourly prices, annualised |
| **Options-implied σ** | Deribit DVOL (BTC/ETH), where available |

The headline signal is the **fee/vol ratio** `feeAPR ÷ (σ_realized²/8)`:

- **ratio ≥ 1.25 → attractive** — fees comfortably beat the volatility cost.
- **0.8 ≤ ratio < 1.25 → fair**.
- **ratio < 0.8 → unattractive** — volatility costs more than the fees pay.

The math lives in [`internal/analyzer`](internal/analyzer/analyzer.go) and is
covered by unit tests.

## Volatility methods, ratio-price fix, ideal ranges, expected edge & junk filter

All of the following derive from a **single OHLCV request per pool** (14 days of
hourly bars, fetched once); the shorter windows and the range estimator are
sliced/computed from that one dataset, never an extra API call. A counting test
([`geckoterminal_test.go`](internal/datasource/geckoterminal_test.go)) enforces
this budget.

### Realized-volatility methods (Phase 1) — `internal/analyzer/volatility.go`

Switchable from the dashboard (`σ method`), recomputing every downstream metric
from the chosen sigma:

| Method | Window | Estimator |
|---|---|---|
| `close7d` *(default, unchanged)* | 7 days | close-to-close log-return stdev × √(periods/yr) |
| `close14d` | 14 days | same, longer window |
| `gk` | 14 days | **Garman-Klass** range estimator |

Garman-Klass (Garman & Klass, 1980) reuses the open/high/low/close already in
each bar, `v = ½(ln H/L)² − (2ln2−1)(ln C/O)²`, annualised as `√(periods · mean v)`.
It recovers the same sigma with **~5× lower sample variance** than close-to-close
(verified in `volatility_test.go`) at zero extra data cost. Illiquid pools (gaps,
zero-volume bars) return a clear *not enough data* signal rather than a garbage
number.

**Volatility uses the pool RATIO price.** The OHLCV is requested in the pool's
native-token terms (`currency=token&token=base`), i.e. the base token priced in
the quote token — the pool's own price, which is what LVR depends on. For a
token/stablecoin pair this ≈ the USD price (verdicts unchanged), but for a
token/token pair like cbBTC/WETH it is the (lower) *relative* volatility of two
correlated assets, not cbBTC's standalone USD volatility. Current USD prices used
for valuation come from the pool's reported token prices (single values), not
this series. All methods still derive from the one stored series
(`ratio_test.go` proves ratio vol < base-USD vol, and equality for a stable quote).

### Ideal range bands (Phase 2) — `internal/analyzer/ranges.go`

Under GBM the log-price over `T` days is Normal with stdev `s = σ·√(T/365)`
(volatility grows with √time). The lognormal containment band is, for `z` ∈ {1,2}:

```
up   = exp(+z·s) − 1        down = 1 − exp(−z·s)
```

≈68% (`z=1`) and ≈95% (`z=2`) containment; the up move is slightly larger in
magnitude than the down move because price is `exp(log-price)`. **These bands are
a containment target, not a profit-optimal width** (see Expected edge below). The
band width `k` (±1σ / ±2σ) and horizon `T` are selectable. The tracked position
also shows its **actual** range as notional prices, computed from
`tickLower/tickUpper` + decimals via `internal/v3math`
(`price = 1.0001^tick · 10^(dec0−dec1)`) — display-only, zero network/chain
calls; within-range reads 0% at the lower tick, 100% at the upper.

### Expected net edge at a fixed ±kσ band — `internal/analyzer/concentrated.go`

Earlier this searched for a profit-*optimal* width, but the model amplifies fees
by the concentration factor `C` without penalising a tight band with lost fees,
so the search always pinned to a corner (grid floor / ceiling) — useless. It is
replaced by the expected net edge at the **fixed ±kσ band** (the same band as
above), with fee capture **weighted by time in range**:

- log half-width `δ = k·σ·√(T/365)`; concentration `C(δ) = 1/(1 − e^(−δ/2))`.
- containment proxy `p = erf(k/√2)` (≈0.68 for k=1, ≈0.95 for k=2) — the fraction
  of time the price is within the band. *This is the key correction*: a tighter
  band has a larger `C` but a smaller `p`, so tighter no longer wins automatically.

```
expectedFee = C·feeAPR·p
expectedLVR = C·(σ²/8)·p
exitCost    = (σ²/δ²)·(rebalanceCost + boundaryLoss(δ))      boundaryLoss(δ)=s(s−1)/(s²+1), s=e^(δ/2)
netEdge     = expectedFee − expectedLVR − exitCost
```

`σ²/δ² = 365/(k²T)` is the boundary-exit rate (the BM first-exit rate), so the
whole expression is **bounded and comparable across pools** (no corner blowup).
`p` is a containment proxy for time-in-range; the number is an estimate, not a
guaranteed return. Computed for k=1 and k=2 per method; the dashboard column and
detail card react to the ±kσ, horizon and σ-method toggles.

**Assumptions / parameters** (configurable, with defaults): horizon `T`
(`DefaultHorizonDays` = 1 day), `rebalanceCost` (`DefaultRebalanceCost` = 5 bps),
GBM zero drift. feeAPR is the pool's current yield independent of position size,
so a low-vol / high-fee pool still shows a large (but finite, clamp-independent)
idealised edge — an upper bound. The reusable primitives (`ConcentrationFactor`,
`boundaryLoss`, the same-width invariant) are unit-tested in `concentrated_test.go`.

### Junk-pool filter (scanner) — `internal/scanner`

GeckoTerminal's pools endpoint only sorts by volume, so dust and wash-trade pools
(tiny TVL, enormous volume → impossible fee APRs) sit at the top. They are dropped
in-code on the already-fetched batch (no extra API call) **before** analysis and
ranking:

- `MIN_TVL_USD` — drop pools with reserves below this (default `$250,000`).
- `MAX_TURNOVER` — drop pools whose 24h-volume/TVL exceeds this (default `20`);
  absurd turnover is the signature of wash trading and the source of impossible
  yields. Legitimate large pools clear both. (`filter_test.go`.)

### Offline reconciliation

A captured-shape GeckoTerminal fixture
([`testdata/`](internal/datasource/testdata)) is fed from disk and reconciled
against a hand calculation (feeAPR exact) and the generating sigma
([`reconcile_test.go`](internal/datasource/reconcile_test.go)). An offline
integration smoke test ([`api_test.go`](internal/api/api_test.go)) asserts the
new API fields are present, finite and within plausible bounds.

## Tracked position & delta hedge

A concentrated-liquidity position is net **long** its volatile leg (e.g. the
WETH in a WETH/USDC pool), so its value falls when ETH falls. The tracker
neutralises that directional risk by shorting the same number of units on a
perpetual future:

```
target short (ETHUSDT) = WETH units currently held in the LP
drift                  = target short − current short
```

The dashboard's **Tracked position & hedge** panel shows, side by side:

- the live LP — pool, fee tier, token amounts, tick range, in/out-of-range,
  USD value;
- the perp short — venue/symbol, target vs. current size, drift (how far the
  hedge has slipped as the position rebalanced), entry/mark price and PnL;
- the fee-vs-volatility verdict for that exact pool — **fee-implied σ** vs
  **realized σ** vs **Deribit IV**, fee APR and net edge.

`internal/position` defines a `Tracker` with a **live** implementation
(`internal/lp` reader + GeckoTerminal pricing + Deribit IV + `internal/binance`
hedge read) and a **demo** implementation. The hedge is **read/advisory by
default** (`HEDGE_DRY_RUN=true`); actually placing orders stays in the
standalone `cmd/hedge` tool.

## Architecture

```
cmd/server          HTTP server entrypoint (config via env)
internal/scanner    periodic scan loop + position refresh + snapshot store
internal/analyzer   the fee-vs-volatility model (+ tests)
internal/position   single-position tracker + perp hedge (live/demo, + tests)
internal/datasource pool/price providers behind a common interface:
                      - geckoterminal.go  live pools + single-pool + OHLCV
                      - deribit.go        options-implied vol (DVOL)
                      - demo.go           synthetic data (offline/CI)
internal/lp         on-chain Aerodrome position reader (Uniswap V3-style)
internal/binance    Binance futures read + short sync
internal/api        JSON API + static file serving
web/assets          dashboard (HTML/CSS/JS, embedded via go:embed)
```

The scanner runs one scan on startup, then every `SCAN_INTERVAL`. Results are
held in memory and served to the dashboard, which polls every 5s and shows a
live countdown to the next scan. A **Scan now** button forces an out-of-band
scan.

## Running

```bash
# Offline demo data (default) — no API keys, no network needed:
go run ./cmd/server
# open http://localhost:8080

# Live data from GeckoTerminal (pools/OHLCV) + Deribit (implied vol):
DATA_SOURCE=live SCAN_INTERVAL=10m go run ./cmd/server
```

> Live mode needs outbound access to `api.geckoterminal.com` and
> `www.deribit.com`. Both are public and key-free, but GeckoTerminal rate-limits
> to ~30 req/min, so the source self-throttles.

### Configuration (environment variables)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DATA_SOURCE` | `demo` | `demo` or `live` |
| `SCAN_INTERVAL` | `10m` | wait between scans (`30s`, `5m`, `1h`, …) |
| `POOLS_PER_CHAIN` | `8` | pools kept per chain |
| `MIN_TVL_USD` | `250000` | drop pools with reserves below this (junk filter) |
| `MAX_TURNOVER` | `20` | drop pools with 24h-volume/TVL above this (wash-trade filter) |
| `CHAINS` | built-in set | comma-separated slugs, e.g. `eth,base,arbitrum` |
| `TRACK_TOKEN_ID` | `71002035` | Aerodrome position token ID to track |
| `HEDGE_DRY_RUN` | `true` | `false` lets `cmd/hedge` actually place orders |
| `RPC_URL` | — | Base RPC (required for **live** position tracking) |
| `BINANCE_TESTNET_API_KEY` / `_SECRET` | — | optional; enables live hedge read |
| `GECKOTERMINAL_URL` | public API | override base URL (proxy) |
| `DERIBIT_URL` | public API | override base URL (proxy) |

Default chains span L1s (Ethereum, BNB Chain, Solana, Avalanche) and L2s
(Arbitrum, Base, Optimism, Polygon).

## API

| Method | Path | Description |
|---|---|---|
| GET | `/api/health` | liveness |
| GET | `/api/meta` | source, interval, chains, verdict counts |
| GET | `/api/pools` | all analyzed pools (filters: `?chainKind=L1\|L2`, `?chain=base`, `?verdict=attractive`) |
| GET | `/api/pools/{chain}/{address}` | single pool detail |
| GET | `/api/position` | the tracked LP position + hedge |
| POST | `/api/scan` | trigger an immediate scan |

## Tests

```bash
go test ./...
```

## Standalone CLIs

The server reuses these; they also run on their own:

- `main.go` — print a single Aerodrome position by token ID (tick range,
  in/out-of-range, token amounts).
- `cmd/hedge` — read the position and **sync** a Binance futures short to match
  its WETH leg (`HEDGE_DRY_RUN=false` to actually trade).
- `cmd/binance-check` — Binance testnet connectivity + balances.
- `internal/v3math` — Uniswap V3 tick/sqrt-price math (shared).

These require an `RPC_URL` (and Binance testnet keys for the hedger) in `.env`.
