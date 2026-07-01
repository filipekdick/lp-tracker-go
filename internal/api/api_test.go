package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/onchain"
	"github.com/filipekdick/lp-tracker-go/internal/position"
	"github.com/filipekdick/lp-tracker-go/internal/scanner"
)

// TestIntegrationSmokeDemo starts the scanner in demo mode (the offline default),
// serves the API, and asserts the Phase 1-3 fields are present on /api/pools and
// /api/position: finite, non-negative where they should be, within plausible
// bounds. No network, no RPC.
func TestIntegrationSmokeDemo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := scanner.New(
		scanner.Config{Chains: datasource.DefaultChains[:2], PerChain: 4, PositionInterval: time.Hour},
		datasource.NewDemo(1),
		datasource.DemoImpliedVol{},
		position.NewDemoTracker(1),
		onchain.NoopProber{},
	)
	go sc.Run(ctx)

	// Wait for the first demo scan to populate the snapshot.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(sc.Snapshot().Pools) > 0 && sc.Snapshot().Position != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	ts := httptest.NewServer(New(sc, nil).Handler())
	defer ts.Close()

	// ---- /api/pools ----------------------------------------------------------
	var pools struct {
		Pools []map[string]any `json:"pools"`
	}
	getJSON(t, ts.URL+"/api/pools", &pools)
	if len(pools.Pools) == 0 {
		t.Fatal("no pools returned")
	}
	for _, p := range pools.Pools {
		a, _ := p["analysis"].(map[string]any)
		if a == nil {
			t.Fatal("pool missing analysis")
		}
		methods, _ := a["methods"].(map[string]any)
		if methods == nil {
			t.Fatal("analysis missing methods (Phase 1)")
		}
		for _, key := range []string{"close7d", "close14d", "gk"} {
			m, _ := methods[key].(map[string]any)
			if m == nil {
				t.Fatalf("missing method %s", key)
			}
			if ok, _ := m["ok"].(bool); !ok {
				continue // not-ok methods are allowed to omit metrics
			}
			rv := num(t, m, "realizedVol")
			if rv < 0 || math.IsNaN(rv) || math.IsInf(rv, 0) {
				t.Fatalf("%s realizedVol implausible: %v", key, rv)
			}
			// Phase 2 bands present and ordered (up magnitude >= down).
			finite(t, m, "band1Up")
			finite(t, m, "band1Down")
			if num(t, m, "band1Up") < num(t, m, "band1Down") {
				t.Fatalf("%s band1 up < down", key)
			}
			// Honest, pool-data-only readouts that replaced the band-edge column.
			// Volatility headroom = breakeven sigma / realized sigma: present,
			// finite, non-negative.
			finite(t, m, "volHeadroom")
			if hr := num(t, m, "volHeadroom"); hr < 0 {
				t.Fatalf("%s volHeadroom negative: %v", key, hr)
			}
			// Expected time in range = k^2*T days at the method's horizon: present,
			// finite, and exactly T (k=1) / 4T (k=2).
			finite(t, m, "expTimeInRange1")
			finite(t, m, "expTimeInRange2")
			hd := num(t, m, "horizonDays")
			if t1 := num(t, m, "expTimeInRange1"); math.Abs(t1-hd) > 1e-9 {
				t.Fatalf("%s expTimeInRange1 %v != horizon %v", key, t1, hd)
			}
			if t2 := num(t, m, "expTimeInRange2"); math.Abs(t2-4*hd) > 1e-9 {
				t.Fatalf("%s expTimeInRange2 %v != 4*horizon %v", key, t2, 4*hd)
			}
			// The removed band-edge fields must be gone.
			for _, removed := range []string{"expEdge1", "expEdge2"} {
				if _, ok := m[removed]; ok {
					t.Fatalf("%s still exposes removed field %s", key, removed)
				}
			}
		}
	}

	// ---- /api/position -------------------------------------------------------
	var pos struct {
		Tracking bool           `json:"tracking"`
		Position map[string]any `json:"position"`
	}
	getJSON(t, ts.URL+"/api/position", &pos)
	if !pos.Tracking {
		t.Fatal("expected tracking position in demo mode")
	}
	// Phase 2 bonus: range prices from ticks, no network.
	lo := num(t, pos.Position, "rangeLowerPrice")
	hi := num(t, pos.Position, "rangeUpperPrice")
	within := num(t, pos.Position, "rangePositionPct")
	if !(lo > 0 && hi > lo) {
		t.Fatalf("range prices implausible: lower=%v upper=%v", lo, hi)
	}
	if within < 0 || within > 100 {
		t.Fatalf("within-range pct out of bounds: %v", within)
	}
	a, _ := pos.Position["analysis"].(map[string]any)
	if methods, _ := a["methods"].(map[string]any); methods == nil {
		t.Fatal("position analysis missing methods")
	}
}

func getJSON(t *testing.T, url string, dst any) {
	t.Helper()
	r, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

func num(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	v, ok := m[key].(float64)
	if !ok {
		t.Fatalf("missing/invalid numeric field %q", key)
	}
	return v
}

// TestTrackedEndpoint checks the runtime tracked-token editor API: GET returns
// the current set, POST replaces it (deduping/dropping invalid IDs), and the
// tracked position reflects the new set. No network.
func TestTrackedEndpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := scanner.New(
		scanner.Config{Chains: datasource.DefaultChains[:1], PerChain: 2, PositionInterval: time.Hour},
		datasource.NewDemo(1),
		datasource.DemoImpliedVol{},
		position.NewDemoTracker(71002035),
		onchain.NoopProber{},
	)
	go sc.Run(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && sc.Snapshot().Position == nil {
		time.Sleep(20 * time.Millisecond)
	}

	ts := httptest.NewServer(New(sc, nil).Handler())
	defer ts.Close()

	// GET the current set (one tracked ID → one synthetic position).
	var got struct {
		TokenIDs []int64 `json:"tokenIds"`
	}
	getJSON(t, ts.URL+"/api/tracked", &got)
	if len(got.TokenIDs) != 1 || got.TokenIDs[0] != 71002035 {
		t.Fatalf("initial tracked = %v, want [71002035]", got.TokenIDs)
	}

	// POST a new set, with a duplicate and an invalid (0) that must be dropped.
	body := `{"tokenIds":[100,200,200,0,300]}`
	r, err := http.Post(ts.URL+"/api/tracked", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/tracked: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", r.StatusCode)
	}
	var set struct {
		TokenIDs []int64 `json:"tokenIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&set); err != nil {
		t.Fatalf("decode POST resp: %v", err)
	}
	if len(set.TokenIDs) != 3 || set.TokenIDs[0] != 100 || set.TokenIDs[2] != 300 {
		t.Fatalf("post-set tracked = %v, want [100 200 300]", set.TokenIDs)
	}

	// The tracked position should now show three synthetic positions.
	var pos struct {
		Tracking bool `json:"tracking"`
		Position struct {
			Positions []map[string]any `json:"positions"`
		} `json:"position"`
	}
	getJSON(t, ts.URL+"/api/position", &pos)
	if len(pos.Position.Positions) != 3 {
		t.Fatalf("position count = %d, want 3", len(pos.Position.Positions))
	}

	// An empty set must be rejected (never wipe tracking).
	r2, err := http.Post(ts.URL+"/api/tracked", "application/json", strings.NewReader(`{"tokenIds":[]}`))
	if err != nil {
		t.Fatalf("POST empty: %v", err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty POST status = %d, want 400", r2.StatusCode)
	}
}

func finite(t *testing.T, m map[string]any, key string) {
	t.Helper()
	v := num(t, m, key)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		t.Fatalf("field %q not finite: %v", key, v)
	}
}
