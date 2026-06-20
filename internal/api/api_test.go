package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/datasource"
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
			// Expected net edge at the ±1σ and ±2σ bands: present, finite, with a
			// sane containment proxy in (0,1).
			for _, ek := range []string{"expEdge1", "expEdge2"} {
				e, _ := m[ek].(map[string]any)
				if e == nil {
					t.Fatalf("%s missing %s", key, ek)
				}
				finite(t, e, "netEdgeApr")
				cont := num(t, e, "containment")
				if cont <= 0 || cont >= 1 {
					t.Fatalf("%s %s containment out of (0,1): %v", key, ek, cont)
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

func finite(t *testing.T, m map[string]any, key string) {
	t.Helper()
	v := num(t, m, key)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		t.Fatalf("field %q not finite: %v", key, v)
	}
}
