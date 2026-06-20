package analyzer

import (
	"math"
	"math/rand"
	"testing"
)

const hoursPerYear = 24 * 365

// gbmBars generates n OHLCV bars from a geometric Brownian motion with the given
// annualised volatility and zero drift. Within each bar the high and low are
// drawn from the EXACT continuous-time extreme distributions of a Brownian
// bridge pinned at the bar's open and close, rather than from coarse intrabar
// sub-sampling. This makes Garman-Klass unbiased (sub-sampling would understate
// the continuous high-low range and bias it low). The seed makes every series
// reproducible, so the recovered volatility is deterministic for a given seed.
//
// Brownian-bridge extremes (diffusion variance w over the bar, endpoints a, b in
// log-price): for h ≥ max(a,b), P(High > h) = exp(-2(h-a)(h-b)/w). Inverting with
// U~Uniform(0,1) gives the closed-form draws below (symmetric for the low).
func gbmBars(seed int64, n int, p0, annualVol, ppy float64) []OHLCV {
	rng := rand.New(rand.NewSource(seed))
	w := annualVol * annualVol / ppy // log-price variance over one bar
	sd := math.Sqrt(w)
	x := math.Log(p0)
	bars := make([]OHLCV, 0, n)
	for i := 0; i < n; i++ {
		a := x
		b := x + (-0.5*w + rng.NormFloat64()*sd) // log close, with GBM drift term
		// High: solve (h-a)(h-b)=q, q=-(w/2)ln U, take the upper root.
		qh := -(w / 2) * math.Log(rng.Float64())
		high := 0.5 * ((a + b) + math.Sqrt((a-b)*(a-b)+4*qh))
		// Low: solve (a-l)(b-l)=q', take the lower root.
		ql := -(w / 2) * math.Log(rng.Float64())
		low := 0.5 * ((a + b) - math.Sqrt((a-b)*(a-b)+4*ql))
		bars = append(bars, OHLCV{
			Time:   int64(i),
			Open:   math.Exp(a),
			High:   math.Exp(high),
			Low:    math.Exp(low),
			Close:  math.Exp(b),
			Volume: 1000,
		})
		x = b
	}
	return bars
}

// TestVolRecovery feeds a long, fixed-seed GBM OHLC series with a known sigma
// and checks each estimator recovers it.
//
// Tolerance: we use a long series (8760 bars = 1 simulated year) so the
// estimator's sampling error is small. Standard error of a vol estimate is
// ~sigma/sqrt(2N); with N≈8760 that is ~0.8% relative, i.e. ~0.005 absolute for
// sigma=0.6. We allow 0.03 absolute, a handful of sampling SEs. The intrabar
// high/low are drawn from the exact Brownian-bridge extremes, so Garman-Klass is
// unbiased here. The seed is fixed so this is reproducible.
func TestVolRecovery(t *testing.T) {
	const (
		trueSigma = 0.60
		tol       = 0.03
	)
	bars := gbmBars(1, hoursPerYear, 1000, trueSigma, hoursPerYear)

	c2c, ok := CloseToCloseVol(bars, hoursPerYear)
	if !ok {
		t.Fatal("close-to-close returned not-ok on a clean series")
	}
	if math.Abs(c2c-trueSigma) > tol {
		t.Fatalf("close-to-close vol = %.4f, want %.2f ± %.2f", c2c, trueSigma, tol)
	}

	gk, ok := GarmanKlassVol(bars, hoursPerYear)
	if !ok {
		t.Fatal("garman-klass returned not-ok on a clean series")
	}
	if math.Abs(gk-trueSigma) > tol {
		t.Fatalf("garman-klass vol = %.4f, want %.2f ± %.2f", gk, trueSigma, tol)
	}
}

// TestMethodWindowRecovery checks the windowed method dispatch (7d/14d) also
// recovers a known sigma from a 14-day hourly series.
func TestMethodWindowRecovery(t *testing.T) {
	const (
		trueSigma = 0.50
		tol       = 0.10 // shorter windows (168/336 bars) => larger sampling error
	)
	bars := gbmBars(7, 14*24, 2000, trueSigma, hoursPerYear)
	for _, m := range AllMethods() {
		got, ok := RealizedVolByMethod(bars, hoursPerYear, m)
		if !ok {
			t.Fatalf("%s: not-ok on a clean 14d series", m)
		}
		if math.Abs(got-trueSigma) > tol {
			t.Fatalf("%s vol = %.4f, want %.2f ± %.2f", m, got, trueSigma, tol)
		}
	}
}

// TestGarmanKlassMoreEfficient verifies the headline efficiency property: across
// many independent seeds, the Garman-Klass estimator's sample variance about the
// true sigma is strictly lower than close-to-close on the same bars.
func TestGarmanKlassMoreEfficient(t *testing.T) {
	const (
		trueSigma = 0.60
		seeds     = 300
		nBars     = 14 * 24 // 14 days hourly
	)
	var c2c, gk []float64
	for s := int64(0); s < seeds; s++ {
		bars := gbmBars(1000+s, nBars, 1000, trueSigma, hoursPerYear)
		if v, ok := CloseToCloseVol(bars, hoursPerYear); ok {
			c2c = append(c2c, v)
		}
		if v, ok := GarmanKlassVol(bars, hoursPerYear); ok {
			gk = append(gk, v)
		}
	}
	vc := sampleVar(c2c)
	vg := sampleVar(gk)
	if !(vg < vc) {
		t.Fatalf("Garman-Klass not more efficient: var(GK)=%.6g, var(C2C)=%.6g", vg, vc)
	}
	t.Logf("efficiency: var(C2C)=%.6g var(GK)=%.6g ratio=%.2f", vc, vg, vc/vg)
}

// sampleVar is the (population) sample variance about the data's own mean — the
// spread of the estimator, which is the efficiency quantity of interest.
func sampleVar(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var s float64
	for _, x := range xs {
		d := x - mean
		s += d * d
	}
	return s / float64(len(xs))
}

// TestVolGracefulDegradation checks that gaps / zero-volume bars yield a clear
// "not enough data" signal (ok=false) and never NaN, Inf, or a silent number.
func TestVolGracefulDegradation(t *testing.T) {
	// All zero-volume bars: nothing usable.
	zero := make([]OHLCV, 50)
	for i := range zero {
		zero[i] = OHLCV{Open: 100, High: 101, Low: 99, Close: 100, Volume: 0}
	}
	for _, fn := range []struct {
		name string
		f    func([]OHLCV, float64) (float64, bool)
	}{{"c2c", CloseToCloseVol}, {"gk", GarmanKlassVol}} {
		v, ok := fn.f(zero, hoursPerYear)
		if ok {
			t.Fatalf("%s: expected not-ok on all-zero-volume bars", fn.name)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("%s: returned non-finite %v", fn.name, v)
		}
	}

	// Too few valid bars (below minValidBars).
	few := []OHLCV{
		{Open: 100, High: 101, Low: 99, Close: 100, Volume: 1},
		{Open: 100, High: 102, Low: 99, Close: 101, Volume: 1},
	}
	if _, ok := CloseToCloseVol(few, hoursPerYear); ok {
		t.Fatal("c2c: expected not-ok with < minValidBars")
	}
	if _, ok := GarmanKlassVol(few, hoursPerYear); ok {
		t.Fatal("gk: expected not-ok with < minValidBars")
	}

	// A flat but fully-valid series is a legitimate zero, not "not enough data".
	flat := make([]OHLCV, 50)
	for i := range flat {
		flat[i] = OHLCV{Open: 100, High: 100, Low: 100, Close: 100, Volume: 1}
	}
	if v, ok := CloseToCloseVol(flat, hoursPerYear); !ok || v != 0 {
		t.Fatalf("c2c flat: got (%v, %v), want (0, true)", v, ok)
	}
}

// TestClose7dRegression pins that the new windowed 7-day close-to-close method
// equals the legacy RealizedVolatility over the identical closes, so the default
// view is unchanged.
func TestClose7dRegression(t *testing.T) {
	bars := gbmBars(42, 14*24, 1500, 0.7, hoursPerYear)

	// Legacy path over the last 7 days of closes.
	w := windowBars(bars, hoursPerYear, 7)
	closes := make([]float64, len(w))
	for i, b := range w {
		closes[i] = b.Close
	}
	legacy := RealizedVolatility(closes, hoursPerYear)

	got, ok := RealizedVolByMethod(bars, hoursPerYear, MethodClose7d)
	if !ok {
		t.Fatal("close7d not-ok")
	}
	if math.Abs(got-legacy) > 1e-12 {
		t.Fatalf("close7d = %.15f, legacy = %.15f (should be identical)", got, legacy)
	}
}
