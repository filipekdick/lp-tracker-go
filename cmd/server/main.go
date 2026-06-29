// Command server runs the liquidity-pool volatility analyzer: it periodically
// scans the most active pools across several L1 and L2 chains, scores each one
// on how well its fee yield compensates for the underlying assets' volatility,
// and serves the results as a JSON API plus a web dashboard.
//
// Configuration (all optional) via environment variables:
//
//	PORT              HTTP listen port (default 8080)
//	DATA_SOURCE       "live" (GeckoTerminal + Deribit) or "demo" (default "demo")
//	SCAN_INTERVAL     informational pool-scan cadence, e.g. "10m" (default 10m)
//	POSITION_INTERVAL how often to refresh the LP position + hedge (default 3m)
//	POOLS_PER_CHAIN   pools to keep per chain (default 8)
//	CHAINS            comma-separated chain slugs to scan (default: built-in set)
//	GECKOTERMINAL_URL override GeckoTerminal API base URL
//	DERIBIT_URL       override Deribit API base URL
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"

	"github.com/filipekdick/lp-tracker-go/internal/api"
	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/hedger"
	"github.com/filipekdick/lp-tracker-go/internal/lp"
	"github.com/filipekdick/lp-tracker-go/internal/position"
	"github.com/filipekdick/lp-tracker-go/internal/scanner"
	"github.com/filipekdick/lp-tracker-go/web"
)

func main() {
	_ = godotenv.Load()

	cfg := scanner.Config{
		Chains:           resolveChains(os.Getenv("CHAINS")),
		PerChain:         envInt("POOLS_PER_CHAIN", 8),
		Interval:         envDuration("SCAN_INTERVAL", 10*time.Minute),
		PositionInterval: envDuration("POSITION_INTERVAL", 3*time.Minute),
		MinTVLUSD:        envFloat("MIN_TVL_USD", scanner.DefaultMinTVLUSD),
		MaxTurnover:      envFloat("MAX_TURNOVER", scanner.DefaultMaxTurnover),
	}

	// Hedge quote preference: token/USDC by default, token/USDT only as a
	// fallback when no USDC perp is listed. "usdt" forces the USDT contract.
	position.HedgeQuotePreference = strings.ToLower(envStr("HEDGE_QUOTE", "usdc"))

	source, implied := buildSources()
	log.Printf("data source: %s | %d chains | %d pools/chain | position every %s | pool scan: manual trigger only",
		source.Name(), len(cfg.Chains), cfg.PerChain, cfg.PositionInterval)

	tracker := buildTracker(source, implied)
	if tracker != nil {
		ids := trackTokenIDs()
		log.Printf("position tracking: %s | token IDs %v | hedge quote: %s", tracker.Name(), ids, position.HedgeQuotePreference)
	}

	sc := scanner.New(cfg, source, implied, tracker)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if strings.ToLower(envStr("DATA_SOURCE", "demo")) == "live" {
		runStartupSummary(ctx)
	}

	go sc.Run(ctx)

	server := &http.Server{
		Addr:              ":" + envStr("PORT", "8080"),
		Handler:           api.New(sc, web.Assets()).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("dashboard listening on http://localhost%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

// buildSources picks the pool/implied-vol providers based on DATA_SOURCE.
func buildSources() (datasource.Source, datasource.ImpliedVolSource) {
	switch strings.ToLower(envStr("DATA_SOURCE", "demo")) {
	case "live", "geckoterminal", "gecko":
		if os.Getenv("CG_API_KEY") != "" {
			log.Println("GeckoTerminal: using CoinGecko demo API key (faster polling ~28 req/min)")
		} else {
			log.Println("GeckoTerminal: no CG_API_KEY set — throttling to ~9 req/min (set CG_API_KEY to go faster)")
		}
		return datasource.NewGeckoTerminal(os.Getenv("GECKOTERMINAL_URL"), os.Getenv("CG_API_KEY")),
			datasource.NewDeribit(os.Getenv("DERIBIT_URL"))
	default:
		return datasource.NewDemo(time.Now().UnixNano()), datasource.DemoImpliedVol{}
	}
}

// Aerodrome (Base) contract addresses for the on-chain position reader, and the
// default position to track.
const (
	aeroFactoryAddress = "0x5e7BB104d84c7CB9B682AaC2F3d509f5F406809A"
	defaultTokenID     = 71002035
)

// buildTracker builds the single-position tracker. In demo mode it synthesises
// a position; in live mode it reads an Aerodrome position on Base from RPC,
// prices it via GeckoTerminal, pulls implied vol from Deribit and (optionally)
// reads the Binance hedge. A misconfigured live tracker degrades to the demo
// tracker rather than crashing the server.
func buildTracker(source datasource.Source, implied datasource.ImpliedVolSource) position.Tracker {
	tokenIDs := trackTokenIDs()

	if strings.ToLower(envStr("DATA_SOURCE", "demo")) != "live" {
		return position.NewDemoTracker(tokenIDs[0])
	}

	rpcURL := os.Getenv("RPC_URL")
	gecko, okGecko := source.(*datasource.GeckoTerminal)
	if rpcURL == "" || !okGecko {
		log.Println("live position tracking needs RPC_URL and the live data source; using demo tracker")
		return position.NewDemoTracker(tokenIDs[0])
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Printf("could not dial RPC for position tracking (%v); using demo tracker", err)
		return position.NewDemoTracker(tokenIDs[0])
	}

	nfpmAddr := envStr("NFPM_ADDRESS", "0x827922686190790b37229fd06084350E74485b72")
	reader := lp.NewReader(client, nfpmAddr, aeroFactoryAddress)

	// Binance hedge is optional: without keys the hedge is advisory.
	var bn *binance.Client
	apiKey := envStr("BINANCE_TESTNET_API_KEY", os.Getenv("BINANCE_API_KEY"))
	apiSecret := envStr("BINANCE_TESTNET_API_SECRET", os.Getenv("BINANCE_API_SECRET"))
	if apiKey != "" && apiSecret != "" {
		bn = binance.Connect(apiKey, apiSecret)
	}

	base := datasource.Chain{Slug: "base", Display: "Base", Kind: datasource.L2}
	dryRun := envStr("HEDGE_DRY_RUN", "true") != "false"

	lower := envFloat("HEDGE_LIMIT_THRESHOLD", envFloat("HEDGE_DRIFT_THRESHOLD", 0.03))
	upper := envFloat("HEDGE_MARKET_THRESHOLD", envFloat("HEDGE_DRIFT_THRESHOLD", 0.06))
	strategy := &hedger.Strategy{LowerThreshold: lower, UpperThreshold: upper}

	return position.NewLiveTracker(reader, gecko, implied, bn, tokenIDs, base, dryRun, strategy)
}

// trackTokenIDs reads the LP positions to track. TRACK_TOKEN_IDS takes a
// comma-separated list (e.g. "71002035,71002099"); the legacy single-value
// TRACK_TOKEN_ID is honoured for backward compatibility. Always returns at least
// one ID (the built-in default) so callers can index [0] safely.
func trackTokenIDs() []int64 {
	raw := envStr("TRACK_TOKEN_IDS", os.Getenv("TRACK_TOKEN_ID"))
	if strings.TrimSpace(raw) == "" {
		return []int64{defaultTokenID}
	}
	var ids []int64
	seen := map[int64]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		ids = append(ids, n)
	}
	if len(ids) == 0 {
		return []int64{defaultTokenID}
	}
	return ids
}

// resolveChains maps a comma-separated slug list onto the known chain set,
// falling back to the default spread of L1s and L2s.
func resolveChains(raw string) []datasource.Chain {
	if strings.TrimSpace(raw) == "" {
		return datasource.DefaultChains
	}
	known := make(map[string]datasource.Chain, len(datasource.DefaultChains))
	for _, c := range datasource.DefaultChains {
		known[c.Slug] = c
	}
	var out []datasource.Chain
	for _, slug := range strings.Split(raw, ",") {
		slug = strings.TrimSpace(slug)
		if c, ok := known[slug]; ok {
			out = append(out, c)
		} else if slug != "" {
			// Unknown slug: assume L1 so it can still be scanned live.
			out = append(out, datasource.Chain{Slug: slug, Display: slug, Kind: datasource.L1})
		}
	}
	if len(out) == 0 {
		return datasource.DefaultChains
	}
	return out
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func runStartupSummary(ctx context.Context) {
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Println("[startup] connecting to Base RPC... error: RPC_URL is not set")
		return
	}

	fmt.Printf("[startup] connecting to Base RPC... ")
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		fmt.Printf("error querying chain ID: %v\n", err)
		return
	}
	fmt.Printf("ok (chain id %s)\n", chainID.String())

	// Reading positions
	tokenIDs := trackTokenIDs()
	nfpmAddrStr := envStr("NFPM_ADDRESS", "0x827922686190790b37229fd06084350E74485b72")
	reader := lp.NewReader(client, nfpmAddrStr, aeroFactoryAddress)
	for _, tokenID := range tokenIDs {
		fmt.Printf("[startup] reading LP position %d on NFPM %s... ", tokenID, nfpmAddrStr)
		report, err := reader.ReadPosition(tokenID)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		fmt.Printf("ok\n")
		inRangeStr := "out of range"
		if report.InRange {
			inRangeStr = "in range"
		}
		fmt.Printf("[startup]   pool: %s/%s  amounts: %.1f %s / %.2f %s  (%s)\n",
			report.Symbol0, report.Symbol1,
			report.Amount0, report.Symbol0,
			report.Amount1, report.Symbol1,
			inRangeStr,
		)
	}

	// Connecting to Binance
	apiKey := envStr("BINANCE_TESTNET_API_KEY", os.Getenv("BINANCE_API_KEY"))
	apiSecret := envStr("BINANCE_TESTNET_API_SECRET", os.Getenv("BINANCE_API_SECRET"))
	if apiKey == "" || apiSecret == "" {
		log.Println("[startup] connecting to Binance futures testnet... skipping hedge, advisory only")
	} else {
		fmt.Printf("[startup] connecting to Binance futures testnet... ")
		bn := binance.Connect(apiKey, apiSecret)
		err := bn.SyncTime(ctx)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("ok\n")
			balances, err := bn.GetBalances(ctx)
			if err != nil {
				fmt.Printf("[startup]   error reading balances: %v\n", err)
			} else {
				for _, bal := range balances {
					if bal.Asset == "USDT" {
						fmt.Printf("[startup]   account: USDT balance %s (available %s)\n", bal.Balance, bal.AvailableBalance)
					}
				}
			}

			openPos, err := bn.GetOpenPositions(ctx)
			if err != nil {
				fmt.Printf("[startup]   error reading open positions: %v\n", err)
			} else {
				for _, pos := range openPos {
					side := "LONG"
					if pos.Size < 0 {
						side = "SHORT"
					}
					fmt.Printf("[startup]   open positions: %s %s %.2f @ entry %.2f PnL %+.2f\n",
						pos.Symbol, side, math.Abs(pos.Size), pos.EntryPrice, pos.UnrealizedPnL,
					)
				}
			}
		}
	}

	log.Println("[startup] ready — starting scan + position loops")
}
