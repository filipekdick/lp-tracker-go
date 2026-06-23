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

// TestSameWidthInvariant is the guard for the core math correction: at any given
// width both the fee yield and the LVR are amplified by the SAME C, so their
// in-range ratio is invariant to the width and equals the full-range ratio.
// Because that ratio is C-invariant, so is the breakeven volatility — the
// position's own width cancels.
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

// TestContainment pins the time-in-range containment confidence p at k=1 and k=2.
func TestContainment(t *testing.T) {
	if p := Containment(1); math.Abs(p-0.6826894921) > 1e-9 {
		t.Fatalf("p(k=1) = %.10f, want ≈0.6827", p)
	}
	if p := Containment(2); math.Abs(p-0.9544997361) > 1e-9 {
		t.Fatalf("p(k=2) = %.10f, want ≈0.9545", p)
	}
}

// TestBreakevenSigma is the corrected, concentration-invariant breakeven: it is
// sqrt(8*feeAPR) for any pool, independent of any band width, and equals the
// fee-implied volatility to a tight tolerance.
func TestBreakevenSigma(t *testing.T) {
	for _, fee := range []float64{0.0, 0.01, 0.1, 0.3, 1.0} {
		got := BreakevenSigma(fee)
		want := math.Sqrt(8 * fee)
		if math.Abs(got-want) > 1e-12 {
			t.Fatalf("BreakevenSigma(%v) = %.12f, want sqrt(8*fee) = %.12f", fee, got, want)
		}
		// Equals the fee-implied sigma exactly — they are the same quantity.
		if math.Abs(got-FeeImpliedVolatility(fee)) > 1e-12 {
			t.Fatalf("BreakevenSigma(%v) = %.12f != FeeImpliedVolatility = %.12f", fee, got, FeeImpliedVolatility(fee))
		}
	}
}

// TestVolHeadroom checks the volatility-space attractiveness ratio against a
// hand computation and against the square root of the old fee/vol APR ratio,
// and that a stable/stable low-vol pool yields a large but finite headroom.
func TestVolHeadroom(t *testing.T) {
	const (
		feeAPR = 0.18
		sigma  = 0.42
	)
	got := VolHeadroom(feeAPR, sigma)

	// Hand-computed: sqrt(8*feeAPR)/sigma = breakeven sigma / realized sigma.
	want := math.Sqrt(8*feeAPR) / sigma
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("VolHeadroom = %.12f, want sqrt(8*fee)/sigma = %.12f", got, want)
	}
	// Equals breakeven (fee-implied) sigma divided by realized sigma.
	if math.Abs(got-FeeImpliedVolatility(feeAPR)/sigma) > 1e-12 {
		t.Fatalf("VolHeadroom != feeImplied/realized")
	}
	// Equals the SQUARE ROOT of the old fee/vol APR ratio feeAPR/(sigma^2/8).
	oldRatio := feeAPR / LVRCost(sigma)
	if math.Abs(got-math.Sqrt(oldRatio)) > 1e-12 {
		t.Fatalf("VolHeadroom %.12f != sqrt(old fee/vol ratio) %.12f", got, math.Sqrt(oldRatio))
	}

	// Stable/stable, very low realized vol: large but finite, no blow-up.
	h := VolHeadroom(0.30, 1e-4)
	if math.IsNaN(h) || math.IsInf(h, 0) {
		t.Fatalf("low-vol headroom not finite: %v", h)
	}
	if !(h > 100) {
		t.Fatalf("low-vol headroom should be large, got %v", h)
	}
	// No volatility signal (or no fees): 0, never NaN/Inf.
	if VolHeadroom(0.30, 0) != 0 {
		t.Fatalf("zero realized vol should give 0 headroom, got %v", VolHeadroom(0.30, 0))
	}
	if VolHeadroom(0, 0.5) != 0 {
		t.Fatalf("zero fees should give 0 headroom, got %v", VolHeadroom(0, 0.5))
	}
}

// TestExpectedTimeInRange checks the closed form k^2*T (days) for k=1 and k=2
// over a couple of horizons, and that it is invariant to sigma — the underlying
// first-passage mean delta^2/sigma^2 is the same for any sigma because the band
// delta = k*sigma*sqrt(T/365) is itself sized by sigma.
func TestExpectedTimeInRange(t *testing.T) {
	cases := []struct{ k, T, want float64 }{
		{1, 1, 1}, {1, 7, 7}, {1, 0.5, 0.5},
		{2, 1, 4}, {2, 7, 28}, {2, 3, 12},
	}
	for _, c := range cases {
		if got := ExpectedTimeInRangeDays(c.k, c.T); math.Abs(got-c.want) > 1e-12 {
			t.Fatalf("ExpectedTimeInRangeDays(k=%v,T=%v) = %v, want k^2*T = %v", c.k, c.T, got, c.want)
		}
	}

	// Sigma-invariance: the raw first-passage mean (delta^2/sigma^2 in years,
	// converted to days) equals k^2*T for ANY sigma at fixed k and T.
	for _, k := range []float64{1, 2} {
		for _, T := range []float64{1, 7} {
			want := ExpectedTimeInRangeDays(k, T)
			var prev float64
			for i, sigma := range []float64{0.05, 0.3, 0.9, 2.0} {
				delta := k * sigma * math.Sqrt(T/365.0)
				days := (delta * delta / (sigma * sigma)) * 365.0 // first-passage mean, days
				if math.Abs(days-want) > 1e-9 {
					t.Fatalf("first-passage days=%v != k^2*T=%v (k=%v T=%v sigma=%v)", days, want, k, T, sigma)
				}
				if i > 0 && math.Abs(days-prev) > 1e-9 {
					t.Fatalf("time-in-range varied with sigma: %v vs %v (k=%v T=%v)", days, prev, k, T)
				}
				prev = days
			}
		}
	}

	// Degenerate guards.
	if ExpectedTimeInRangeDays(0, 1) != 0 || ExpectedTimeInRangeDays(1, 0) != 0 {
		t.Fatal("non-positive k or T should give 0")
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

// TestMethodsHeadroomAndTimeInRange checks that buildMethods exposes the new
// honest readouts: headroom = breakeven sigma / realized sigma per method, and
// expected time in range = k^2*T (sigma-independent) at the default horizon.
func TestMethodsHeadroomAndTimeInRange(t *testing.T) {
	// Two synthetic price paths with clearly different realized vols.
	mk := func(step float64) []OHLCV {
		bars := make([]OHLCV, 0, 400)
		price := 2000.0
		up := true
		for i := 0; i < 400; i++ {
			if up {
				price *= 1 + step
			} else {
				price *= 1 - step
			}
			up = !up
			o := price
			c := price
			bars = append(bars, OHLCV{Open: o, High: price * (1 + step/2), Low: price * (1 - step/2), Close: c, Volume: 1000})
		}
		return bars
	}
	lowVol := Analyze(Input{FeeTier: 0.003, TVLUSD: 5_000_000, Volume24hUSD: 8_000_000, PeriodsPerYear: 8760, Bars: mk(0.001)})
	highVol := Analyze(Input{FeeTier: 0.003, TVLUSD: 5_000_000, Volume24hUSD: 8_000_000, PeriodsPerYear: 8760, Bars: mk(0.02)})

	for name, res := range map[string]Result{"low": lowVol, "high": highVol} {
		for m, mr := range res.Methods {
			if !mr.OK {
				continue
			}
			// Headroom is exactly breakeven/realized.
			want := mr.FeeImpliedVol / mr.RealizedVol
			if math.Abs(mr.VolHeadroom-want) > 1e-12 {
				t.Fatalf("%s/%s headroom=%v want %v", name, m, mr.VolHeadroom, want)
			}
			// Time in range = k^2 * horizon, independent of sigma.
			if math.Abs(mr.ExpTimeInRange1-mr.HorizonDays) > 1e-12 {
				t.Fatalf("%s/%s timeInRange1=%v want T=%v", name, m, mr.ExpTimeInRange1, mr.HorizonDays)
			}
			if math.Abs(mr.ExpTimeInRange2-4*mr.HorizonDays) > 1e-12 {
				t.Fatalf("%s/%s timeInRange2=%v want 4T=%v", name, m, mr.ExpTimeInRange2, 4*mr.HorizonDays)
			}
		}
	}

	// Same k and horizon => identical time-in-range across two different sigmas.
	for m := range lowVol.Methods {
		l, h := lowVol.Methods[m], highVol.Methods[m]
		if !l.OK || !h.OK {
			continue
		}
		if l.RealizedVol == h.RealizedVol {
			continue // need genuinely different sigmas to make the point
		}
		if l.ExpTimeInRange1 != h.ExpTimeInRange1 || l.ExpTimeInRange2 != h.ExpTimeInRange2 {
			t.Fatalf("%s time-in-range varied with sigma: low(%v,%v) high(%v,%v)", m,
				l.ExpTimeInRange1, l.ExpTimeInRange2, h.ExpTimeInRange1, h.ExpTimeInRange2)
		}
	}
}
