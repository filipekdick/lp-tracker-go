# LP Vol Analyzer

A crypto liquidity-pool analyzer that auto-scans the most active pools across
multiple L1 and L2 chains and judges, for each one, whether the fees it pays
actually compensate liquidity providers for the **price volatility** of the
pooled assets.

It ships as a single Go binary: a background scanner + JSON API + a zero-build
web dashboard. The repo also still contains the original Aerodrome position
tracker and Binance futures hedger (see [Other tools](#other-tools)).

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

## Architecture

```
cmd/server          HTTP server entrypoint (config via env)
internal/scanner    periodic scan loop + in-memory snapshot store
internal/analyzer   the fee-vs-volatility model (+ tests)
internal/datasource pool/price providers behind a common interface:
                      - geckoterminal.go  live pools + hourly OHLCV
                      - deribit.go        options-implied vol (DVOL)
                      - demo.go           synthetic data (offline/CI)
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
| POST | `/api/scan` | trigger an immediate scan |

## Tests

```bash
go test ./...
```

## Other tools

The repo predates the analyzer and also includes:

- `main.go` — reads a single Aerodrome (Uniswap V3-style) position by token ID
  and reports its tick range, in/out-of-range status and token amounts.
- `cmd/hedge`, `cmd/binance-check` — Binance futures read + delta hedge sync.
- `internal/v3math` — Uniswap V3 tick/sqrt-price math (shared, tested).

These require an `RPC_URL` (and Binance keys for the hedger) in `.env`.
