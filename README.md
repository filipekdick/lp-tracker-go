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
