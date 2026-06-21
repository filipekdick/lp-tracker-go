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

// TestExpectedBandEdgeFeeTermHasNoC is the central correction: the fee term is
// feeAPR*p with NO concentration factor, even when C is large. The IL term keeps
// C. Switching k changes both delta and p.
func TestExpectedBandEdgeFeeTermHasNoC(t *testing.T) {
	// Small sigma -> tight band -> large C, the regime that used to blow up.
	in := BandEdgeInput{FeeAPR: 0.30, Sigma: 0.05, K: 1, HorizonDays: 1, RebalanceCost: 0.0005}
	r1 := ExpectedBandEdge(in)

	if !(r1.ConcentrationC > 100) {
		t.Fatalf("expected a large C for this regime, got %.3f", r1.ConcentrationC)
	}
	// Fee term must equal feeAPR*p exactly — C must NOT appear.
	wantFee := in.FeeAPR * Containment(1)
	if math.Abs(r1.ExpectedFeeAPR-wantFee) > 1e-12 {
		t.Fatalf("fee term = %.12f, want feeAPR*p = %.12f (no C)", r1.ExpectedFeeAPR, wantFee)
	}
	// Sanity: the (wrong) C-amplified value would be hundreds of % — confirm we
	// are nowhere near it.
	if r1.ExpectedFeeAPR > in.FeeAPR {
		t.Fatalf("fee term %.4f exceeds feeAPR %.4f — C leaked in", r1.ExpectedFeeAPR, in.FeeAPR)
	}
	// IL term keeps C exactly.
	wantLVR := ConcentrationFactor(r1.Delta) * LVRCost(in.Sigma) * Containment(1)
	if math.Abs(r1.ExpectedLVRAPR-wantLVR) > 1e-12 {
		t.Fatalf("IL term = %.12f, want C*(σ²/8)*p = %.12f", r1.ExpectedLVRAPR, wantLVR)
	}
	if math.Abs(r1.NetEdgeAPR-(r1.ExpectedFeeAPR-r1.ExpectedLVRAPR-r1.ExitCostAPR)) > 1e-12 {
		t.Fatal("net edge != fee - lvr - exit")
	}

	in.K = 2
	r2 := ExpectedBandEdge(in)
	if math.Abs(r2.Delta-2*r1.Delta) > 1e-12 {
		t.Fatalf("delta(k=2)=%.6f, want 2*delta(k=1)=%.6f", r2.Delta, 2*r1.Delta)
	}
	if !(r2.Containment > r1.Containment) {
		t.Fatalf("p(k=2)=%.4f should exceed p(k=1)=%.4f", r2.Containment, r1.Containment)
	}
	// Still no C at k=2.
	if math.Abs(r2.ExpectedFeeAPR-in.FeeAPR*Containment(2)) > 1e-12 {
		t.Fatalf("k=2 fee term = %.12f, want feeAPR*p", r2.ExpectedFeeAPR)
	}
}

// TestExpectedBandEdgeBoundedByFeeAPR is the headline boundedness invariant:
// netEdge <= feeAPR for ANY pool, including a stable/stable pool with very small
// sigma (which used to explode to hundreds of thousands of %). With costs off, a
// tiny-sigma pool's edge approaches feeAPR*p — close to feeAPR, not above it.
func TestExpectedBandEdgeBoundedByFeeAPR(t *testing.T) {
	feeAPRs := []float64{0.01, 0.05, 0.20, 0.50, 1.0}
	sigmas := []float64{1e-4, 0.01, 0.05, 0.2, 0.6, 1.2, 2.0}
	ks := []float64{1, 2}
	Ts := []float64{0.5, 1, 7}
	for _, fee := range feeAPRs {
		for _, sig := range sigmas {
			for _, k := range ks {
				for _, T := range Ts {
					r := ExpectedBandEdge(BandEdgeInput{FeeAPR: fee, Sigma: sig, K: k, HorizonDays: T, RebalanceCost: 0.0005})
					if math.IsNaN(r.NetEdgeAPR) || math.IsInf(r.NetEdgeAPR, 0) {
						t.Fatalf("non-finite edge for fee=%v sig=%v k=%v T=%v", fee, sig, k, T)
					}
					// THE invariant: never above feeAPR (tiny epsilon for float slack).
					if r.NetEdgeAPR > fee+1e-12 {
						t.Fatalf("edge %.6f exceeds feeAPR %.6f (fee=%v sig=%v k=%v T=%v)", r.NetEdgeAPR, fee, fee, sig, k, T)
					}
				}
			}
		}
	}

	// Stable/stable, very small sigma, costs off: edge -> feeAPR*p, i.e. close to
	// feeAPR rather than orders of magnitude above it.
	const fee = 0.30
	st := ExpectedBandEdge(BandEdgeInput{FeeAPR: fee, Sigma: 1e-4, K: 1, HorizonDays: 1, RebalanceCost: 0})
	want := fee * Containment(1)
	if math.Abs(st.NetEdgeAPR-want) > 1e-3 {
		t.Fatalf("tiny-sigma edge = %.6f, want ≈ feeAPR*p = %.6f", st.NetEdgeAPR, want)
	}
	if st.NetEdgeAPR > fee {
		t.Fatalf("tiny-sigma edge %.6f exceeds feeAPR %.6f", st.NetEdgeAPR, fee)
	}
}

// TestExpectedBandEdgeILReducesEdge confirms the C-amplified IL term meaningfully
// reduces the edge for a volatile/volatile pool, by an amount that grows with
// sigma.
func TestExpectedBandEdgeILReducesEdge(t *testing.T) {
	const fee = 0.40
	mk := func(sig float64) BandEdgeResult {
		return ExpectedBandEdge(BandEdgeInput{FeeAPR: fee, Sigma: sig, K: 1, HorizonDays: 1, RebalanceCost: 0})
	}
	lo := mk(0.3)
	hi := mk(0.9)
	// IL term is a real haircut below the fee term.
	if !(lo.ExpectedLVRAPR > 0) || !(lo.ExpectedFeeAPR-lo.ExpectedLVRAPR < lo.ExpectedFeeAPR) {
		t.Fatalf("IL term should reduce the edge, got LVR=%.6f", lo.ExpectedLVRAPR)
	}
	// Higher sigma -> larger IL haircut.
	if !(hi.ExpectedLVRAPR > lo.ExpectedLVRAPR) {
		t.Fatalf("IL haircut should grow with sigma: lo=%.6f hi=%.6f", lo.ExpectedLVRAPR, hi.ExpectedLVRAPR)
	}
}

// TestBreakevenSigma checks the concentration-aware breakeven volatility: it
// reduces to sqrt(8*feeAPR) at full range (C->1) and is strictly lower for a
// tighter (higher-C) band.
func TestBreakevenSigma(t *testing.T) {
	const fee = 0.10
	full := FeeImpliedVolatility(fee) // sqrt(8*feeAPR), the C=1 reference
	// Large delta => C ≈ 1 => sigma* ≈ sqrt(8*feeAPR).
	wide := BreakevenSigma(fee, 40)
	if math.Abs(wide-full) > 1e-6 {
		t.Fatalf("breakeven at full range = %.9f, want %.9f", wide, full)
	}
	// Tighter band (smaller delta, larger C) => strictly lower sigma*.
	tight := BreakevenSigma(fee, 0.05)
	if !(tight < full) {
		t.Fatalf("tighter-band breakeven %.6f should be < full-range %.6f", tight, full)
	}
	// And it must match sqrt(8*feeAPR/C) exactly.
	wantTight := math.Sqrt(8 * fee / ConcentrationFactor(0.05))
	if math.Abs(tight-wantTight) > 1e-12 {
		t.Fatalf("breakeven = %.12f, want sqrt(8*fee/C) = %.12f", tight, wantTight)
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
