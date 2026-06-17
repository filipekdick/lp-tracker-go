// Command server runs the liquidity-pool volatility analyzer: it periodically
// scans the most active pools across several L1 and L2 chains, scores each one
// on how well its fee yield compensates for the underlying assets' volatility,
// and serves the results as a JSON API plus a web dashboard.
//
// Configuration (all optional) via environment variables:
//
//	PORT              HTTP listen port (default 8080)
//	DATA_SOURCE       "live" (GeckoTerminal + Deribit) or "demo" (default "demo")
//	SCAN_INTERVAL     wait between scans, e.g. "10m", "30s" (default 10m)
//	POOLS_PER_CHAIN   pools to keep per chain (default 8)
//	CHAINS            comma-separated chain slugs to scan (default: built-in set)
//	GECKOTERMINAL_URL override GeckoTerminal API base URL
//	DERIBIT_URL       override Deribit API base URL
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/filipekdick/lp-tracker-go/internal/api"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/scanner"
	"github.com/filipekdick/lp-tracker-go/web"
)

func main() {
	_ = godotenv.Load()

	cfg := scanner.Config{
		Chains:   resolveChains(os.Getenv("CHAINS")),
		PerChain: envInt("POOLS_PER_CHAIN", 8),
		Interval: envDuration("SCAN_INTERVAL", 10*time.Minute),
	}

	source, implied := buildSources()
	log.Printf("data source: %s | %d chains | %d pools/chain | scan every %s",
		source.Name(), len(cfg.Chains), cfg.PerChain, cfg.Interval)

	sc := scanner.New(cfg, source, implied)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
		return datasource.NewGeckoTerminal(os.Getenv("GECKOTERMINAL_URL")),
			datasource.NewDeribit(os.Getenv("DERIBIT_URL"))
	default:
		return datasource.NewDemo(time.Now().UnixNano()), datasource.DemoImpliedVol{}
	}
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
