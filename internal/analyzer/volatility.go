package analyzer

// This file adds selectable realized-volatility estimators (Phase 1).
//
// Every estimator is computed from a single slice of OHLCV bars (one
// GeckoTerminal request per pool). The longest window (14 days) is fetched once
// and the shorter windows are derived by slicing the tail of that slice, so no
// estimator costs an extra API call.
//
// Three methods are exposed:
//
//   - close7d : close-to-close log-return volatility over the last 7 days.
//     This is the historical default and is identical to RealizedVolatility.
//   - close14d: the same estimator over the last 14 days.
//   - gk      : the Garman-Klass range estimator over the last 14 days, which
//     uses the open/high/low/close already present in each bar to produce a
//     lower-variance estimate of the same volatility at no extra data cost.
//
// Structure note (for a later, already-planned correction): every estimator
// takes a plain []OHLCV slice and a periods-per-year scalar. The planned switch
// to pool-ratio prices (base priced in quote instead of base priced in USD)
// only needs to feed a different []OHLCV slice into the same functions, still
// from the single OHLCV call, so it slots in without touching the math here.

import "math"

// OHLCV is one bar of price history, oldest-first when in a slice. Fields mirror
// the GeckoTerminal row [timestamp, open, high, low, close, volume].
type OHLCV struct {
	Time   int64   `json:"t"`
	Open   float64 `json:"o"`
	High   float64 `json:"h"`
	Low    float64 `json:"l"`
	Close  float64 `json:"c"`
	Volume float64 `json:"v"`
}

// VolMethod identifies a realized-volatility estimator the dashboard can switch
// between.
type VolMethod string

const (
	MethodClose7d     VolMethod = "close7d"
	MethodClose14d    VolMethod = "close14d"
	MethodGarmanKlass VolMethod = "gk"

	// DefaultMethod keeps the historical 7-day close-to-close estimator as the
	// default so existing behaviour is unchanged.
	DefaultMethod = MethodClose7d
)

// methodWindowDays is the trailing window, in days, each method is computed over.
var methodWindowDays = map[VolMethod]float64{
	MethodClose7d:     7,
	MethodClose14d:    14,
	MethodGarmanKlass: 14,
}

// MethodLabel returns a short human-readable label for the dashboard.
func MethodLabel(m VolMethod) string {
	switch m {
	case MethodClose7d:
		return "Close/close · 7d"
	case MethodClose14d:
		return "Close/close · 14d"
	case MethodGarmanKlass:
		return "Garman-Klass · 14d"
	default:
		return string(m)
	}
}

// AllMethods lists the supported methods in display order (default first).
func AllMethods() []VolMethod {
	return []VolMethod{MethodClose7d, MethodClose14d, MethodGarmanKlass}
}

// windowBars returns the trailing bars covering `days` at the given sampling
// rate. n = round(days * periodsPerYear / 365). It returns the whole slice when
// it is shorter than the window so short histories degrade to "use what we
// have" rather than returning nothing.
func windowBars(bars []OHLCV, periodsPerYear, days float64) []OHLCV {
	if periodsPerYear <= 0 || days <= 0 {
		return bars
	}
	n := int(math.Round(days * periodsPerYear / 365.0))
	if n <= 0 || n >= len(bars) {
		return bars
	}
	return bars[len(bars)-n:]
}

// validBars keeps only bars with strictly positive OHLC and non-zero volume.
// Zero-volume or non-positive bars indicate an illiquid pool with gaps; dropping
// them lets range estimators "degrade gracefully" instead of returning garbage
// from a stale or empty candle.
func validBars(bars []OHLCV) []OHLCV {
	out := make([]OHLCV, 0, len(bars))
	for _, b := range bars {
		if b.Open > 0 && b.High > 0 && b.Low > 0 && b.Close > 0 &&
			b.High >= b.Low && b.Volume > 0 {
			out = append(out, b)
		}
	}
	return out
}

// minValidBars is the smallest number of usable bars we will produce a
// volatility estimate from. Below this we report "not enough data" (ok=false)
// rather than a number computed from one or two candles.
const minValidBars = 12

// CloseToCloseVol returns the annualised close-to-close volatility of the given
// bars and whether the estimate is usable. It is the standard estimator:
//
//	sigma = stdev(log(C_t / C_{t-1})) * sqrt(periodsPerYear)
//
// ok is false when there are too few usable bars; a genuinely flat (zero-vol)
// series returns (0, true).
func CloseToCloseVol(bars []OHLCV, periodsPerYear float64) (float64, bool) {
	if periodsPerYear <= 0 {
		return 0, false
	}
	vb := validBars(bars)
	if len(vb) < minValidBars {
		return 0, false
	}

	returns := make([]float64, 0, len(vb)-1)
	for i := 1; i < len(vb); i++ {
		returns = append(returns, math.Log(vb[i].Close/vb[i-1].Close))
	}
	if len(returns) < 2 {
		return 0, false
	}

	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	var sq float64
	for _, r := range returns {
		d := r - mean
		sq += d * d
	}
	variance := sq / float64(len(returns)-1) // sample (Bessel-corrected) variance
	sigma := math.Sqrt(variance) * math.Sqrt(periodsPerYear)
	if math.IsNaN(sigma) || math.IsInf(sigma, 0) {
		return 0, false
	}
	return sigma, true
}

// GarmanKlassVol returns the annualised Garman-Klass range volatility.
//
// Garman & Klass (1980) showed that, for a driftless diffusion sampled into
// OHLC bars, the per-bar variance is estimated with far lower sampling error by
// combining the high-low range with the open-close move:
//
//	v_t = 0.5 * (ln(H/L))^2 - (2*ln2 - 1) * (ln(C/O))^2
//
// Averaging v_t over the bars and annualising gives
//
//	sigma_GK = sqrt( periodsPerYear * mean(v_t) )
//
// Because each bar contributes its own intrabar extremes, GK has roughly 7x the
// statistical efficiency of close-to-close for the same data, so it recovers the
// true sigma with a much smaller sample variance (verified in the tests). We use
// the zero-drift, no-overnight-jump form, which matches how the bars are
// sampled (continuous hourly trading, no gap term).
func GarmanKlassVol(bars []OHLCV, periodsPerYear float64) (float64, bool) {
	if periodsPerYear <= 0 {
		return 0, false
	}
	vb := validBars(bars)
	if len(vb) < minValidBars {
		return 0, false
	}

	const c = 2*math.Ln2 - 1 // ≈ 0.386
	var sum float64
	for _, b := range vb {
		hl := math.Log(b.High / b.Low)
		co := math.Log(b.Close / b.Open)
		sum += 0.5*hl*hl - c*co*co
	}
	meanVar := sum / float64(len(vb))
	// The GK term can in principle go slightly negative on a single bar; over a
	// window the mean is non-negative for real data, but clamp to be safe so we
	// never take the sqrt of a negative number.
	if meanVar < 0 {
		meanVar = 0
	}
	sigma := math.Sqrt(periodsPerYear * meanVar)
	if math.IsNaN(sigma) || math.IsInf(sigma, 0) {
		return 0, false
	}
	return sigma, true
}

// RealizedVolByMethod slices the trailing window for the method and dispatches
// to the matching estimator. It is the single entry point the rest of the code
// uses, so adding a method (or the planned ratio-price correction) is local.
func RealizedVolByMethod(bars []OHLCV, periodsPerYear float64, m VolMethod) (float64, bool) {
	days, ok := methodWindowDays[m]
	if !ok {
		days = methodWindowDays[DefaultMethod]
	}
	w := windowBars(bars, periodsPerYear, days)
	switch m {
	case MethodGarmanKlass:
		return GarmanKlassVol(w, periodsPerYear)
	default: // close7d, close14d
		return CloseToCloseVol(w, periodsPerYear)
	}
}

// ClosesToBars adapts a plain close-price series into OHLCV bars (O=H=L=C). It
// lets the close-to-close estimators run on legacy close-only inputs; the range
// estimator is meaningless on such data and is not used there.
func ClosesToBars(closes []float64) []OHLCV {
	bars := make([]OHLCV, 0, len(closes))
	for _, c := range closes {
		bars = append(bars, OHLCV{Open: c, High: c, Low: c, Close: c, Volume: 1})
	}
	return bars
}
