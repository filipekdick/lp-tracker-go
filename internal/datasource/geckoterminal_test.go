package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
)

// TestGeckoSingleOHLCVCallPerPool is the rate-limit guarantee: no matter how
// many volatility methods are computed downstream, GeckoTerminal must be hit at
// most once for OHLCV per pool. The whole 14-day window is fetched in that one
// call and every method is derived from it.
//
// Hermetic: served entirely from an httptest server, no network.
func TestGeckoSingleOHLCVCallPerPool(t *testing.T) {
	const perChain = 3

	var mu sync.Mutex
	ohlcvHits := map[string]int{} // address -> count

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/ohlcv/"):
			// .../pools/{address}/ohlcv/hour — must request native-token (ratio)
			// terms after the Part A fix.
			if q := r.URL.Query(); q.Get("currency") != "token" || q.Get("token") != "base" {
				t.Errorf("ohlcv request not in native-token terms: %s", r.URL.RawQuery)
			}
			addr := ohlcvAddress(path)
			mu.Lock()
			ohlcvHits[addr]++
			mu.Unlock()
			writeOHLCV(w, 336)
		case strings.HasSuffix(path, "/pools"):
			writePoolsList(w, perChain)
		default:
			http.Error(w, "unexpected path "+path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	g := NewGeckoTerminal(srv.URL, "")
	g.reqGap = 0 // disable throttle so the test is fast (same-package access)

	pools, err := g.TopPools(context.Background(), []Chain{{Slug: "base", Display: "Base", Kind: L2}}, perChain)
	if err != nil {
		t.Fatalf("TopPools: %v", err)
	}
	if len(pools) != perChain {
		t.Fatalf("got %d pools, want %d", len(pools), perChain)
	}

	// Every pool must have been enriched with the full 14-day window, and the
	// OHLCV endpoint must have been hit exactly once per pool.
	if len(ohlcvHits) != perChain {
		t.Fatalf("OHLCV hit for %d distinct pools, want %d", len(ohlcvHits), perChain)
	}
	for addr, n := range ohlcvHits {
		if n != 1 {
			t.Fatalf("OHLCV endpoint hit %d times for pool %s, want exactly 1", n, addr)
		}
	}
	for _, p := range pools {
		if len(p.OHLC) != 336 {
			t.Fatalf("pool %s has %d bars, want 336", p.Address, len(p.OHLC))
		}
		// Sanity: all three methods compute from the single slice.
		for _, m := range analyzer.AllMethods() {
			if _, ok := analyzer.RealizedVolByMethod(p.OHLC, p.PeriodsPerYear, m); !ok {
				t.Fatalf("pool %s: method %s returned not-ok on full window", p.Address, m)
			}
		}
	}
}

func ohlcvAddress(path string) string {
	// .../pools/{address}/ohlcv/hour
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "pools" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func writePoolsList(w http.ResponseWriter, n int) {
	var data []map[string]any
	for i := 0; i < n; i++ {
		addr := fmt.Sprintf("0xpool%02d", i)
		data = append(data, map[string]any{
			"id": "base_" + addr,
			"attributes": map[string]any{
				"name":                  "WETH / USDC 0.05%",
				"address":               addr,
				"base_token_price_usd":  "3400",
				"quote_token_price_usd": "1",
				"reserve_in_usd":        "10000000",
				"pool_fee_percentage":   "0.05",
				"volume_usd":            map[string]any{"h24": "5000000"},
			},
			"relationships": map[string]any{
				"base_token":  map[string]any{"data": map[string]any{"id": "base_weth"}},
				"quote_token": map[string]any{"data": map[string]any{"id": "base_usdc"}},
				"dex":         map[string]any{"data": map[string]any{"id": "aerodrome_slipstream"}},
			},
		})
	}
	resp := map[string]any{
		"data": data,
		"included": []map[string]any{
			{"id": "base_weth", "type": "token", "attributes": map[string]any{"symbol": "WETH"}},
			{"id": "base_usdc", "type": "token", "attributes": map[string]any{"symbol": "USDC"}},
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func writeOHLCV(w http.ResponseWriter, n int) {
	rows := make([][]float64, 0, n)
	price := 3400.0
	for i := 0; i < n; i++ {
		ts := float64(1_700_000_000 + i*3600)
		o := price
		c := price * 1.001
		rows = append(rows, []float64{ts, o, c * 1.002, o * 0.998, c, 1_000_000})
		price = c
	}
	resp := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{"ohlcv_list": rows},
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
