package datasource

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
)

// TestLiveReconciliationFixture anchors the pipeline to a captured GeckoTerminal
// response, fed entirely from disk (no network). The pool/OHLCV JSON in testdata
// is shaped exactly like the live API. Expected outputs are pinned against:
//   - feeAPR: a hand calculation, exact.
//   - realized sigma: the GBM volatility the fixture was generated with (0.55),
//     within a tolerance set by sampling error (336 bars => SE ≈ 0.55/sqrt(672)
//     ≈ 0.021; we allow 0.06, ~3 SE), plus an exact golden for regression.
//   - optimal width & net edge: present and finite.
func TestLiveReconciliationFixture(t *testing.T) {
	poolJSON := mustRead(t, "testdata/gecko_pool_weth_usdc.json")
	ohlcvJSON := mustRead(t, "testdata/gecko_ohlcv_weth_usdc.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/ohlcv/") {
			w.Write(ohlcvJSON)
			return
		}
		w.Write(poolJSON)
	}))
	defer srv.Close()

	g := NewGeckoTerminal(srv.URL, "")
	g.reqGap = 0

	chain := Chain{Slug: "base", Display: "Base", Kind: L2}
	rp, err := g.PoolByAddress(context.Background(), chain, "0xb2cc224c1c9fee385f8ad6a55b4d94e92359dc59")
	if err != nil {
		t.Fatalf("PoolByAddress: %v", err)
	}

	// Hand-calculated fee APR: 5,000,000 * 0.0005 / 10,000,000 * 365 = 0.09125.
	res := analyzer.Analyze(analyzer.Input{
		FeeTier:        rp.FeeTier,
		TVLUSD:         rp.TVLUSD,
		Volume24hUSD:   rp.Volume24hUSD,
		Closes:         rp.Closes,
		PeriodsPerYear: rp.PeriodsPerYear,
		Bars:           rp.OHLC,
	})
	const wantFeeAPR = 0.09125
	if math.Abs(res.FeeAPR-wantFeeAPR) > 1e-9 {
		t.Fatalf("feeAPR = %.9f, want %.9f", res.FeeAPR, wantFeeAPR)
	}

	const (
		genSigma = 0.55
		volTol   = 0.06
	)
	for _, m := range analyzer.AllMethods() {
		mr, ok := res.Methods[string(m)]
		if !ok || !mr.OK {
			t.Fatalf("method %s missing/not-ok", m)
		}
		t.Logf("%s: sigma=%.6f optWidth=%.6f netEdge=%.6f", m, mr.RealizedVol, mr.OptimalWidthPct, mr.OptimalNetEdgeAPR)
		if math.Abs(mr.RealizedVol-genSigma) > volTol {
			t.Fatalf("%s sigma = %.4f, want %.2f ± %.2f", m, mr.RealizedVol, genSigma, volTol)
		}
		if mr.OptimalWidthPct <= 0 || math.IsNaN(mr.OptimalWidthPct) || math.IsInf(mr.OptimalWidthPct, 0) {
			t.Fatalf("%s optimal width not finite/positive: %v", m, mr.OptimalWidthPct)
		}
		if math.IsNaN(mr.OptimalNetEdgeAPR) || math.IsInf(mr.OptimalNetEdgeAPR, 0) {
			t.Fatalf("%s optimal net edge not finite: %v", m, mr.OptimalNetEdgeAPR)
		}
	}

	// Exact golden on the default 7-day close-to-close sigma, to catch any
	// regression in parsing or estimation (the fixture is fully deterministic).
	got7d := res.Methods[string(analyzer.MethodClose7d)].RealizedVol
	const golden7d = 0.555490
	if math.Abs(got7d-golden7d) > 1e-5 {
		t.Fatalf("close7d sigma golden = %.6f, want %.6f", got7d, golden7d)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
