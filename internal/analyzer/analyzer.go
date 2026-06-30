// Package analyzer implements the liquidity-range-to-volatility model used to
// judge whether a liquidity pool's fee yield adequately compensates a liquidity
// provider for the price volatility of the underlying assets.
//
// The model is built on the loss-versus-rebalancing (LVR) result for
// constant-product automated market makers (Milionis, Moallemi, Roughgarden &
// Zhang, 2022). For a CFMM tracking an asset whose price follows a geometric
// Brownian motion with annualised volatility sigma, a liquidity provider loses
// value to arbitrageurs at an annual rate of
//
//	LVR_rate = sigma^2 / 8
//
// expressed as a fraction of pooled value. An LP is only profitable when the
// fees collected per year exceed that rebalancing loss. Setting fee income
// equal to the LVR cost and solving for sigma yields the volatility that the
// pool's current fee yield is effectively "pricing in":
//
//	sigma_fees = sqrt(8 * feeAPR)
//
// We call this the fee-implied volatility. Comparing it to the realised
// volatility of the asset (measured from price history) and, where available,
// the options-implied volatility, tells us whether the pool over- or
// under-pays LPs for the risk they take.
package analyzer

import (
	"math"
	"time"
)

// Input is the per-pool data required to run the model. All monetary values are
// in USD and FeeTier is a fraction (e.g. 0.0005 for a 5 bps pool).
type Input struct {
	FeeTier        float64   // pool fee, fractional (0.003 == 0.3%)
	TVLUSD         float64   // total value locked
	Volume24hUSD   float64   // trailing 24h swap volume
	Closes         []float64 // historical close prices, oldest first
	PeriodsPerYear float64   // sampling frequency of Closes (e.g. 8760 for hourly)
	ImpliedVol     float64   // options-implied annualised vol, if known
	HasImplied     bool      // whether ImpliedVol is populated

	// Bars holds the full OHLCV history (oldest first), long enough to cover the
	// longest volatility window (14 days). When present, Analyze computes the
	// selectable per-method volatilities (Phase 1), range bands (Phase 2), the
	// volatility headroom and the informational expected time in range into
	// Result.Methods. It is derived from the same single OHLCV request that
	// produces Closes.
	Bars []OHLCV

	// HorizonDays is the horizon T (days) used for the range bands and the
	// informational expected-time-in-range readout. Zero falls back to
	// DefaultHorizonDays.
	HorizonDays float64
}

// Verdict classifies how a pool's fee yield compares to its volatility cost.
type Verdict string

const (
	VerdictAttractive   Verdict = "attractive"   // fees comfortably beat the volatility cost
	VerdictFair         Verdict = "fair"         // fees roughly match the volatility cost
	VerdictUnattractive Verdict = "unattractive" // volatility cost outweighs fees
	VerdictUnknown      Verdict = "unknown"      // not enough data to judge
)

// Result holds the computed metrics for a pool. Volatilities and rates are
// expressed as annualised fractions (0.5 == 50%).
type Result struct {
	FeeAPR          float64 `json:"feeApr"`          // annualised fee yield on TVL
	PositionFeeAPR  float64 `json:"positionFeeApr"`  // annualised fee yield on specific position
	RealizedVol     float64 `json:"realizedVol"`     // annualised realised volatility
	FeeImpliedVol   float64 `json:"feeImpliedVol"`   // volatility the fee yield prices in
	ImpliedVol      float64 `json:"impliedVol"`      // options-implied volatility (0 if n/a)
	HasImplied      bool    `json:"hasImplied"`      // whether ImpliedVol is meaningful
	LVRCostRealized float64 `json:"lvrCostRealized"` // annual LVR cost at realised vol
	NetEdgeAPR      float64 `json:"netEdgeApr"`      // FeeAPR - LVRCostRealized
	FeeYieldRatio   float64 `json:"feeYieldRatio"`   // FeeAPR / LVRCostRealized
	Verdict         Verdict `json:"verdict"`         // human-readable classification
	Score           float64 `json:"score"`           // ranking score, higher is better

	// Methods holds the selectable per-volatility-method analyses (Phase 1-3),
	// keyed by VolMethod string ("close7d", "close14d", "gk"). It is populated
	// only when Input.Bars is supplied; the top-level fields above are unchanged
	// and remain the default 7-day close-to-close view for backward compatibility.
	Methods map[string]MethodResult `json:"methods,omitempty"`
	// DefaultMethod names the method the top-level fields correspond to.
	DefaultMethod string `json:"defaultMethod,omitempty"`

	// ConcentratedSim is the aggregator-style projected fee APR for a $1,000
	// deposit into a ±kσ band, as the deposit's share of the REAL on-chain
	// in-range liquidity (with a bounded pool-data fallback). It is attached by the
	// scanner (which holds the on-chain prober and a per-scan probe budget), not by
	// Analyze, so the model stays a pure, prober-free function. See
	// concentratedsim.go.
	ConcentratedSim *ConcentratedSim `json:"concentratedSim,omitempty"`
}

// thresholds for the verdict bands, expressed as a fee-yield ratio (fees vs.
// the LVR cost implied by realised volatility).
const (
	ratioAttractive = 1.25
	ratioFair       = 0.80
)

// Analyze runs the full liquidity-range-to-volatility model for one pool.
func Analyze(in Input) Result {
	res := Result{
		ImpliedVol: in.ImpliedVol,
		HasImplied: in.HasImplied,
	}

	res.FeeAPR = FeeAPR(in.Volume24hUSD, in.FeeTier, in.TVLUSD)
	res.FeeImpliedVol = FeeImpliedVolatility(res.FeeAPR)
	res.RealizedVol = RealizedVolatility(in.Closes, in.PeriodsPerYear)

	res.LVRCostRealized = LVRCost(res.RealizedVol)
	res.NetEdgeAPR = res.FeeAPR - res.LVRCostRealized

	if res.LVRCostRealized > 0 {
		res.FeeYieldRatio = res.FeeAPR / res.LVRCostRealized
	}

	res.Verdict = classify(res)
	res.Score = score(res)

	res.DefaultMethod = string(DefaultMethod)
	if len(in.Bars) > 0 {
		res.Methods = buildMethods(in, res.FeeAPR)
	}
	// The concentrated-APR projection (Result.ConcentratedSim) needs the on-chain
	// prober and a context, so it is attached by the scanner after Analyze rather
	// than here — Analyze stays a pure function.
	return res
}

// FeeAPR annualises trailing-24h fee income as a fraction of TVL.
//
//	feeAPR = volume24h * feeTier / TVL * 365
func FeeAPR(volume24hUSD, feeTier, tvlUSD float64) float64 {
	if tvlUSD <= 0 || feeTier <= 0 || volume24hUSD <= 0 {
		return 0
	}
	dailyFees := volume24hUSD * feeTier
	return dailyFees / tvlUSD * 365.0
}

// LVRCost returns the annual loss-versus-rebalancing rate (as a fraction of
// pooled value) for a constant-product AMM at the given annualised volatility:
//
//	LVR = sigma^2 / 8
func LVRCost(sigma float64) float64 {
	if sigma <= 0 {
		return 0
	}
	return sigma * sigma / 8.0
}

// FeeImpliedVolatility inverts the LVR relationship: it returns the volatility
// at which LVR cost would exactly equal the supplied fee APR.
//
//	sigma_fees = sqrt(8 * feeAPR)
func FeeImpliedVolatility(feeAPR float64) float64 {
	if feeAPR <= 0 {
		return 0
	}
	return math.Sqrt(8.0 * feeAPR)
}

// RealizedVolatility computes annualised realised volatility from a series of
// close prices using log returns and the sample standard deviation.
//
//	sigma = stdev(log returns) * sqrt(periodsPerYear)
func RealizedVolatility(closes []float64, periodsPerYear float64) float64 {
	if len(closes) < 3 || periodsPerYear <= 0 {
		return 0
	}

	returns := make([]float64, 0, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		prev, cur := closes[i-1], closes[i]
		if prev <= 0 || cur <= 0 {
			continue
		}
		returns = append(returns, math.Log(cur/prev))
	}
	if len(returns) < 2 {
		return 0
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
	stdev := math.Sqrt(variance)

	return stdev * math.Sqrt(periodsPerYear)
}

// classify assigns a verdict from the fee-yield ratio, falling back to net edge
// when realised volatility (and therefore the ratio) is unavailable.
func classify(r Result) Verdict {
	return classifyRatio(r.FeeAPR, r.FeeYieldRatio, r.RealizedVol)
}

// classifyRatio is the verdict logic factored out so the per-method analyses
// (which carry their own realised vol and ratio) reuse the exact same bands.
func classifyRatio(feeAPR, feeYieldRatio, realizedVol float64) Verdict {
	if feeAPR == 0 {
		return VerdictUnknown
	}
	if realizedVol == 0 {
		// No volatility signal: judge on whether any fees accrue at all.
		if feeAPR > 0 {
			return VerdictFair
		}
		return VerdictUnknown
	}
	switch {
	case feeYieldRatio >= ratioAttractive:
		return VerdictAttractive
	case feeYieldRatio >= ratioFair:
		return VerdictFair
	default:
		return VerdictUnattractive
	}
}

// score is a ranking signal that rewards a high fee-yield ratio while damping
// the contribution of tiny, illiquid pools via a log TVL-free proxy (the ratio
// itself already accounts for relative size). It is monotonic in NetEdgeAPR.
func score(r Result) float64 {
	if r.Verdict == VerdictUnknown {
		return 0
	}
	// Net edge in basis points of annual yield, plus a bonus for the ratio.
	return r.NetEdgeAPR*100 + math.Min(r.FeeYieldRatio, 10)
}

// PositionFeeAPR annualises the fees a single position has accrued over the
// time it has been tracked, as a fraction of the position's current value.
// feesAccruedUSD is the USD value of fees earned since elapsed began.
// elapsed is the time the position has been observed. valueUSD is the current
// position value. Returns 0 when inputs are non-positive or elapsed is too
// short to annualise meaningfully.
func PositionFeeAPR(feesAccruedUSD, valueUSD float64, elapsed time.Duration) float64 {
	if valueUSD <= 0 || feesAccruedUSD <= 0 || elapsed <= 0 {
		return 0
	}
	years := elapsed.Hours() / (24 * 365)
	if years <= 0 {
		return 0
	}
	return (feesAccruedUSD / valueUSD) / years
}
