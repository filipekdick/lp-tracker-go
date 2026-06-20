package analyzer

import (
	"math"
	"testing"
)

// TestConcentrationFactor pins C(delta) at a clean known width, checks that a
// wider range gives a smaller C, and that the full-range limit drives C -> 1.
func TestConcentrationFactor(t *testing.T) {
	// delta = 2*ln2 => e^{-delta/2} = e^{-ln2} = 0.5 => C = 1/(1-0.5) = 2 exactly.
	d := 2 * math.Ln2
	if c := ConcentrationFactor(d); math.Abs(c-2) > 1e-12 {
		t.Fatalf("C(2ln2) = %.12f, want 2", c)
	}
	// Wider range => smaller C.
	if !(ConcentrationFactor(2*d) < ConcentrationFactor(d)) {
		t.Fatalf("wider range should have smaller C")
	}
	// Full-range limit: C -> 1.
	if c := ConcentrationFactor(40); math.Abs(c-1) > 1e-8 {
		t.Fatalf("C(40) = %.12f, want ≈1", c)
	}
}

// TestBoundaryLossDerivation cross-checks the closed-form boundaryLoss against a
// direct value computation: a v3 position centred at p, valued at the upper edge
// (fully one-sided), versus holding the centred inventory to that edge.
func TestBoundaryLossDerivation(t *testing.T) {
	for _, delta := range []float64{0.05, 0.2, 0.5, 1.0} {
		const (
			p = 2000.0
			L = 1.0e6
		)
		sp := math.Sqrt(p)
		spa := math.Sqrt(p * math.Exp(-delta)) // sqrt(lower)
		spb := math.Sqrt(p * math.Exp(delta))  // sqrt(upper)
		pb := p * math.Exp(delta)

		// Centred inventory (price = p, in range).
		x0 := L * (spb - sp) / (sp * spb) // token0
		y0 := L * (sp - spa)              // token1
		// LP value at the upper edge: position is fully token1.
		lpEnd := L * (spb - spa)
		// HODL value at the upper edge (value of the centred inventory).
		hodl := x0*pb + y0
		loss := 1 - lpEnd/hodl

		if got := boundaryLoss(delta); math.Abs(got-loss) > 1e-9 {
			t.Fatalf("delta=%v: boundaryLoss=%.10f, direct=%.10f", delta, got, loss)
		}
	}
	// Monotonic increasing in width.
	if !(boundaryLoss(0.1) < boundaryLoss(0.5)) {
		t.Fatal("boundaryLoss should increase with width")
	}
}

// TestSameWidthInvariant is the guard against the headline trap: at any given
// width both fee yield and LVR are amplified by the SAME C, so their in-range
// ratio is invariant to the width and equals the full-range ratio.
func TestSameWidthInvariant(t *testing.T) {
	const (
		feeAPR = 0.30
		sigma  = 0.60
	)
	want := feeAPR / LVRCost(sigma) // full-range ratio
	for _, d := range []float64{0.01, 0.1, 0.5, 1.0, 3.0} {
		ratio := AmplifiedFeeYield(feeAPR, d) / AmplifiedLVR(sigma, d)
		if math.Abs(ratio-want) > 1e-12 {
			t.Fatalf("delta=%v: in-range ratio=%.12f, want %.12f (must be C-invariant)", d, ratio, want)
		}
	}
}

// TestContainment pins the time-in-range proxy p at k=1 and k=2.
func TestContainment(t *testing.T) {
	if p := Containment(1); math.Abs(p-0.6826894921) > 1e-9 {
		t.Fatalf("p(k=1) = %.10f, want ≈0.6827", p)
	}
	if p := Containment(2); math.Abs(p-0.9544997361) > 1e-9 {
		t.Fatalf("p(k=2) = %.10f, want ≈0.9545", p)
	}
}

// TestExpectedBandEdgeWeighting checks the central correction: the expected fee
// term equals C(delta)*feeAPR*p, and switching k changes both delta and p.
func TestExpectedBandEdgeWeighting(t *testing.T) {
	in := BandEdgeInput{FeeAPR: 0.30, Sigma: 0.60, K: 1, HorizonDays: 1, RebalanceCost: 0.0005}
	r1 := ExpectedBandEdge(in)

	// Fee term must equal C(delta)*feeAPR*p exactly.
	wantFee := ConcentrationFactor(r1.Delta) * in.FeeAPR * Containment(1)
	if math.Abs(r1.ExpectedFeeAPR-wantFee) > 1e-12 {
		t.Fatalf("expected fee = %.12f, want C*feeAPR*p = %.12f", r1.ExpectedFeeAPR, wantFee)
	}
	// LVR term weighted the same way.
	wantLVR := ConcentrationFactor(r1.Delta) * LVRCost(in.Sigma) * Containment(1)
	if math.Abs(r1.ExpectedLVRAPR-wantLVR) > 1e-12 {
		t.Fatalf("expected lvr = %.12f, want C*(σ²/8)*p = %.12f", r1.ExpectedLVRAPR, wantLVR)
	}
	// Net edge is fee - lvr - exit.
	if math.Abs(r1.NetEdgeAPR-(r1.ExpectedFeeAPR-r1.ExpectedLVRAPR-r1.ExitCostAPR)) > 1e-12 {
		t.Fatal("net edge != fee - lvr - exit")
	}

	in.K = 2
	r2 := ExpectedBandEdge(in)
	// delta doubles with k, p rises to ~0.95.
	if math.Abs(r2.Delta-2*r1.Delta) > 1e-12 {
		t.Fatalf("delta(k=2)=%.6f, want 2*delta(k=1)=%.6f", r2.Delta, 2*r1.Delta)
	}
	if !(r2.Containment > r1.Containment) {
		t.Fatalf("p(k=2)=%.4f should exceed p(k=1)=%.4f", r2.Containment, r1.Containment)
	}
}

// TestExpectedBandEdgeBounded checks the estimate is finite/bounded with no
// corner-solution blowup, and that a negative-edge pool yields a sane negative.
//
// Unlike the removed optimiser — whose edge grew without limit as the grid floor
// shrank (a clamp artifact) — this estimate is a single evaluation at the
// principled ±kσ band, so it is finite and depends on no arbitrary clamp. For a
// genuinely low-vol pool the idealised number is still large (the band is tight,
// C is high, and feeAPR is taken independent of position size), but it is finite
// and clamp-independent. The decisive anti-corner property is that raising
// volatility — where the old model still pinned to the floor and blew up — now
// widens the band, drives C→1, and keeps the edge near the full-range value.
func TestExpectedBandEdgeBounded(t *testing.T) {
	// Stablecoin-ish: low vol, healthy fees. Finite and positive.
	stable := ExpectedBandEdge(BandEdgeInput{FeeAPR: 0.20, Sigma: 0.05, K: 1, HorizonDays: 1, RebalanceCost: 0.0005})
	if math.IsNaN(stable.NetEdgeAPR) || math.IsInf(stable.NetEdgeAPR, 0) {
		t.Fatalf("stable net edge not finite: %v", stable.NetEdgeAPR)
	}
	if stable.NetEdgeAPR <= 0 {
		t.Fatalf("high-fee low-vol pool should have positive edge, got %v", stable.NetEdgeAPR)
	}

	// Anti-corner: a HIGH-vol, high-fee pool (where the optimiser used to pin the
	// width at the floor and report an absurd edge in the hundreds) now yields a
	// modest, finite number — the frequent-exit cost from the ±kσ band offsets the
	// amplified fees instead of being ignored.
	hiVol := ExpectedBandEdge(BandEdgeInput{FeeAPR: 0.20, Sigma: 1.0, K: 1, HorizonDays: 1, RebalanceCost: 0.0005})
	if math.IsNaN(hiVol.NetEdgeAPR) || math.IsInf(hiVol.NetEdgeAPR, 0) {
		t.Fatalf("high-vol edge not finite: %v", hiVol.NetEdgeAPR)
	}
	if math.Abs(hiVol.NetEdgeAPR) > 50 {
		t.Fatalf("high-vol high-fee edge %.4f looks like a corner blowup", hiVol.NetEdgeAPR)
	}

	// High-vol, thin fees: LVR dominates -> negative but sane.
	bad := ExpectedBandEdge(BandEdgeInput{FeeAPR: 0.02, Sigma: 1.2, K: 1, HorizonDays: 1, RebalanceCost: 0.0005})
	if bad.NetEdgeAPR >= 0 {
		t.Fatalf("thin-fee high-vol pool should have negative edge, got %v", bad.NetEdgeAPR)
	}
	if math.IsInf(bad.NetEdgeAPR, 0) || bad.NetEdgeAPR < -100 {
		t.Fatalf("negative edge not sane/bounded: %v", bad.NetEdgeAPR)
	}
}

// TestExpectedBandEdgeFlatSeries guards the degenerate zero-sigma case.
func TestExpectedBandEdgeFlatSeries(t *testing.T) {
	r := ExpectedBandEdge(BandEdgeInput{FeeAPR: 0.1, Sigma: 0, K: 1, HorizonDays: 1})
	if math.IsNaN(r.NetEdgeAPR) || math.IsInf(r.NetEdgeAPR, 0) {
		t.Fatalf("flat-series edge not finite: %v", r.NetEdgeAPR)
	}
}

// TestAnalyzeRegressionNoBars guards that Analyze with no OHLC bars behaves
// exactly as before: no Methods map, legacy top-level fields populated.
func TestAnalyzeRegressionNoBars(t *testing.T) {
	closes := make([]float64, 0, 100)
	p := 1000.0
	for i := 0; i < 100; i++ {
		p *= 1.001
		closes = append(closes, p)
	}
	res := Analyze(Input{FeeTier: 0.003, TVLUSD: 2_000_000, Volume24hUSD: 5_000_000, Closes: closes, PeriodsPerYear: 8760})
	if res.Methods != nil {
		t.Fatal("Methods should be nil when no Bars are supplied")
	}
	if res.RealizedVol != RealizedVolatility(closes, 8760) {
		t.Fatal("top-level realized vol changed")
	}
}
