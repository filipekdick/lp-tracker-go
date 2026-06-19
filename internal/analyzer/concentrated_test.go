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

// baseConc is a regime with a clean interior optimum (feeAPR > sigma^2/4 keeps
// the optimum off the boundaries).
func baseConc() ConcentratedInput {
	return ConcentratedInput{FeeAPR: 0.50, Sigma: 0.60, RebalanceCost: 0.05, HorizonDays: 1}
}

// TestDirectionInvariants checks the comparative statics: higher volatility =>
// wider optimum, higher fee => narrower optimum, higher rebalancing cost =>
// wider optimum.
func TestDirectionInvariants(t *testing.T) {
	base := OptimalConcentratedRange(baseConc())

	hiVol := baseConc()
	hiVol.Sigma = 0.90
	if !(OptimalConcentratedRange(hiVol).Delta > base.Delta) {
		t.Fatalf("higher vol should widen optimum: hi=%.5f base=%.5f", OptimalConcentratedRange(hiVol).Delta, base.Delta)
	}

	hiFee := baseConc()
	hiFee.FeeAPR = 0.90
	if !(OptimalConcentratedRange(hiFee).Delta < base.Delta) {
		t.Fatalf("higher fee should narrow optimum: hi=%.5f base=%.5f", OptimalConcentratedRange(hiFee).Delta, base.Delta)
	}

	hiCost := baseConc()
	hiCost.RebalanceCost = 0.15
	if !(OptimalConcentratedRange(hiCost).Delta > base.Delta) {
		t.Fatalf("higher rebalancing cost should widen optimum: hi=%.5f base=%.5f", OptimalConcentratedRange(hiCost).Delta, base.Delta)
	}
}

// TestOptimizerFindsMaximum checks the optimiser lands on the true argmax of the
// net-edge curve. Reference: a brute-force scan at 1e-5 resolution in delta.
// Tolerance: 2e-3 absolute on delta — comfortably above the reference grid step
// yet tight relative to the optimum (~0.05), and the curve is smooth and
// single-peaked in this regime.
func TestOptimizerFindsMaximum(t *testing.T) {
	in := baseConc()
	got := OptimalConcentratedRange(in)

	// Brute-force reference argmax.
	bestD, bestV := minDelta, math.Inf(-1)
	for d := minDelta; d <= maxDelta; d += 1e-5 {
		if v := NetEdgeAtWidth(in, d); v > bestV {
			bestV, bestD = v, d
		}
	}
	if got.Delta <= minDelta+1e-6 || got.Delta >= maxDelta-1e-6 {
		t.Fatalf("expected interior optimum, got delta=%.6f (boundary)", got.Delta)
	}
	if math.Abs(got.Delta-bestD) > 2e-3 {
		t.Fatalf("optimiser delta=%.6f, reference=%.6f (diff %.2e)", got.Delta, bestD, math.Abs(got.Delta-bestD))
	}
	// The reported net edge must be the value at the reported delta.
	if math.Abs(got.NetEdgeAPR-NetEdgeAtWidth(in, got.Delta)) > 1e-12 {
		t.Fatal("reported net edge inconsistent with delta")
	}
}

// TestFullRangeReference checks the full-range net edge and the C->1 limit, so
// the legacy full-range view remains available and consistent.
func TestFullRangeReference(t *testing.T) {
	in := baseConc()
	r := OptimalConcentratedRange(in)
	wantFull := in.FeeAPR - LVRCost(in.Sigma)
	if math.Abs(r.FullRangeNetEdge-wantFull) > 1e-12 {
		t.Fatalf("full-range net edge = %v, want %v", r.FullRangeNetEdge, wantFull)
	}
	// Optimising can only do at least as well as full range.
	if r.NetEdgeAPR < r.FullRangeNetEdge-1e-9 {
		t.Fatalf("optimum %.6f worse than full range %.6f", r.NetEdgeAPR, r.FullRangeNetEdge)
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
