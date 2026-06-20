package analyzer

import (
	"math"
	"testing"
	"time"
)

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestFeeAPR(t *testing.T) {
	// $10M daily volume, 0.3% fee, $5M TVL.
	// daily fees = 30k; APR = 30k/5M*365 = 2.19
	got := FeeAPR(10_000_000, 0.003, 5_000_000)
	if !approx(got, 2.19, 1e-9) {
		t.Fatalf("FeeAPR = %v, want 2.19", got)
	}
	if FeeAPR(0, 0.003, 5_000_000) != 0 {
		t.Fatal("zero volume should give zero APR")
	}
	if FeeAPR(1, 0.003, 0) != 0 {
		t.Fatal("zero TVL should give zero APR (no divide by zero)")
	}
}

func TestLVRCostAndInverse(t *testing.T) {
	// LVR(sigma) = sigma^2/8; FeeImpliedVolatility should invert it.
	sigma := 0.8
	cost := LVRCost(sigma)
	if !approx(cost, 0.08, 1e-9) {
		t.Fatalf("LVRCost(0.8) = %v, want 0.08", cost)
	}
	back := FeeImpliedVolatility(cost)
	if !approx(back, sigma, 1e-9) {
		t.Fatalf("FeeImpliedVolatility(LVRCost(0.8)) = %v, want 0.8", back)
	}
}

func TestRealizedVolatilityZeroForFlatSeries(t *testing.T) {
	flat := []float64{100, 100, 100, 100, 100}
	if v := RealizedVolatility(flat, 8760); v != 0 {
		t.Fatalf("flat series should have zero vol, got %v", v)
	}
}

func TestRealizedVolatilityKnownDeviation(t *testing.T) {
	// Alternating +1%/-1% (approx) log returns => stdev of returns ~0.01.
	// Annualised with 365 daily periods => ~0.01*sqrt(365) ~= 0.191.
	closes := []float64{100}
	up := true
	for i := 0; i < 400; i++ {
		last := closes[len(closes)-1]
		if up {
			closes = append(closes, last*1.01)
		} else {
			closes = append(closes, last*0.99)
		}
		up = !up
	}
	v := RealizedVolatility(closes, 365)
	if v <= 0.1 || v >= 0.3 {
		t.Fatalf("annualised vol = %v, expected roughly 0.19", v)
	}
}

func TestAnalyzeAttractiveWhenFeesBeatVolatility(t *testing.T) {
	// Build a gently trending series with low realised volatility but a pool
	// that earns chunky fees -> should be attractive.
	closes := make([]float64, 0, 200)
	p := 1000.0
	for i := 0; i < 200; i++ {
		p *= 1.0001 // tiny, steady drift => low realised vol
		closes = append(closes, p)
	}
	res := Analyze(Input{
		FeeTier:        0.003,
		TVLUSD:         2_000_000,
		Volume24hUSD:   8_000_000,
		Closes:         closes,
		PeriodsPerYear: 8760,
	})
	if res.FeeAPR <= 0 {
		t.Fatal("expected positive fee APR")
	}
	if res.FeeImpliedVol <= res.RealizedVol {
		t.Fatalf("fee-implied vol (%v) should exceed realised vol (%v) for an attractive pool", res.FeeImpliedVol, res.RealizedVol)
	}
	if res.Verdict != VerdictAttractive {
		t.Fatalf("verdict = %q, want attractive (ratio=%v)", res.Verdict, res.FeeYieldRatio)
	}
}

func TestAnalyzeUnattractiveWhenVolatilityDominates(t *testing.T) {
	// Highly volatile series, thin fees -> unattractive.
	closes := []float64{1000}
	mul := []float64{1.15, 0.85, 1.20, 0.80, 1.18, 0.83, 1.25, 0.78}
	for i := 0; i < 200; i++ {
		closes = append(closes, closes[len(closes)-1]*mul[i%len(mul)])
	}
	res := Analyze(Input{
		FeeTier:        0.0005,
		TVLUSD:         50_000_000,
		Volume24hUSD:   500_000,
		Closes:         closes,
		PeriodsPerYear: 8760,
	})
	if res.Verdict != VerdictUnattractive {
		t.Fatalf("verdict = %q, want unattractive (ratio=%v, realizedVol=%v)", res.Verdict, res.FeeYieldRatio, res.RealizedVol)
	}
	if res.NetEdgeAPR >= 0 {
		t.Fatalf("expected negative net edge, got %v", res.NetEdgeAPR)
	}
}

func TestAnalyzeUnknownWithoutFees(t *testing.T) {
	res := Analyze(Input{FeeTier: 0.003, TVLUSD: 1_000_000, Volume24hUSD: 0})
	if res.Verdict != VerdictUnknown {
		t.Fatalf("verdict = %q, want unknown", res.Verdict)
	}
}

func TestPositionFeeAPR(t *testing.T) {
	// $24 fees on a $7,200 position over 1 hour annualizes to roughly 2,900%.
	apr := PositionFeeAPR(24, 7200, time.Hour)
	// (24/7200) / (1 / (24*365)) = (1/300) * 8760 = 29.2 (which is 2920%)
	if !approx(apr, 29.2, 1e-9) {
		t.Fatalf("PositionFeeAPR = %v, want 29.2", apr)
	}

	if PositionFeeAPR(-5, 7200, time.Hour) != 0 {
		t.Fatal("negative fees should yield zero")
	}
	if PositionFeeAPR(24, -100, time.Hour) != 0 {
		t.Fatal("negative value should yield zero")
	}
	if PositionFeeAPR(24, 7200, 0) != 0 {
		t.Fatal("zero duration should yield zero")
	}
}
