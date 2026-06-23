package analyzer

// concentrated.go holds the concentration primitives and the honest,
// pool-data-only volatility screen that replaced the fragile expected-net-edge-
// at-a-band estimate.
//
// ---------------------------------------------------------------------------
// Key math correction — the breakeven volatility is concentration-invariant.
//
// For any position that is active at the current price, BOTH the fee-yield rate
// and the impermanent-loss (LVR) rate are amplified by the SAME concentration
// factor C relative to a full-range position. Their ratio — and therefore the
// volatility at which the fee rate equals the IL rate (the breakeven) — does
// not depend on C at all. The position's own width cancels.
//
// Concretely, at a band of log half-width delta the in-range fee yield is
// feeAPR*C(delta) and the in-range IL is (sigma^2/8)*C(delta); setting them
// equal, the C(delta) cancels and leaves sigma = sqrt(8*feeAPR). So the
// breakeven volatility computable from POOL DATA ALONE is the full-range
// (C = 1) form:
//
//	sigma_breakeven = sqrt(8 * feeAPR)
//
// This is exactly the existing fee-implied volatility. It is the full-range
// case of the Gemini concentrated breakeven 2*sqrt(feeAPR*w): when fees are
// amplified consistently with IL the width w cancels and you land here.
//
// CAVEAT — the breakeven is OPTIMISTIC. The TRUE breakeven for the pool's
// actual concentration is sqrt(8*feeAPR / C_pool), where C_pool > 1 is the
// pool's existing on-chain concentration (from active liquidity). Because
// C_pool > 1 the real breakeven is LOWER, so sqrt(8*feeAPR) is an upper bound
// and the verdict errs generous. Refining it needs on-chain active-liquidity
// data — a separate future task, deliberately NOT attempted here (no new
// API/RPC calls).
//
// CAVEAT — feeAPR is SWAP FEES ONLY. It excludes farm/gauge incentives (e.g.
// AERO emissions on Aerodrome), so total LP yield on incentivised pools is
// understated and the screen is, again, conservative on those pools.
//
// The earlier expected-net-edge-at-a-band column and the band-dependent
// breakeven sigma*(delta) have been removed: they rested on a fragile
// rebalancing-cost guess and on the double-counting/width error corrected above.
// What remains here are bounded, honest, pool-data-only readouts:
//   - BreakevenSigma:          sqrt(8*feeAPR), the (optimistic) breakeven vol.
//   - VolHeadroom:             sigma_breakeven / realized_sigma, the attractiveness
//                              ratio in volatility space.
//   - ExpectedTimeInRangeDays: k^2 * T, an informational mean time in range.
//
// ConcentrationFactor / AmplifiedFeeYield / AmplifiedLVR are retained only to
// document and test the concentration-invariance above; they no longer feed any
// displayed value or the verdict.

import "math"

// ConcentrationFactor returns C(delta) = 1/(1 - e^{-delta/2}), the factor by
// which a band of log half-width delta amplifies BOTH the in-range fee yield and
// the in-range IL of a position relative to full range. That the SAME C applies
// to both is exactly why it cancels in the breakeven (see file header).
func ConcentrationFactor(delta float64) float64 {
	if delta <= 0 {
		return math.Inf(1)
	}
	return 1.0 / (1.0 - math.Exp(-delta/2.0))
}

// AmplifiedFeeYield returns the in-range fee yield of a concentrated position at
// width delta: feeAPR * C(delta). Used only to demonstrate (in tests) that fees
// and IL share the SAME C, so their ratio — and the breakeven — is C-invariant.
func AmplifiedFeeYield(feeAPR, delta float64) float64 {
	return feeAPR * ConcentrationFactor(delta)
}

// AmplifiedLVR returns the in-range LVR of a concentrated position at width
// delta: (sigma^2/8) * C(delta). Same C as the fee yield — that shared factor is
// the whole point: it cancels in the breakeven.
func AmplifiedLVR(sigma, delta float64) float64 {
	return LVRCost(sigma) * ConcentrationFactor(delta)
}

// BreakevenSigma returns the concentration-invariant breakeven volatility
// computable from pool data alone:
//
//	sigma_breakeven = sqrt(8 * feeAPR)
//
// It is independent of the band width — the C(delta) of the earlier
// band-dependent version cancels (see the file header) — and is identical to
// FeeImpliedVolatility. It is an OPTIMISTIC upper bound: the pool's real
// on-chain concentration C_pool > 1 would lower the true breakeven to
// sqrt(8*feeAPR / C_pool).
func BreakevenSigma(feeAPR float64) float64 {
	return FeeImpliedVolatility(feeAPR)
}

// VolHeadroom reports how many times the breakeven (fee-implied) volatility
// exceeds the volatility that actually happened:
//
//	headroom = sigma_breakeven / realized_sigma = sqrt(8*feeAPR) / sigma
//
// Above 1 means the pool's fees are overpaying for the realized volatility. It
// is the attractiveness ratio in volatility space — the square root of the old
// fee/vol APR ratio feeAPR/(sigma^2/8) — bounded and readable. Returns 0 when
// there is no volatility signal (sigma <= 0) or no fees. A stable/stable
// low-vol pool gives a large but finite headroom; the TVL/turnover junk filter
// remains the guard against absurd pools.
func VolHeadroom(feeAPR, realizedVol float64) float64 {
	if realizedVol <= 0 || feeAPR <= 0 {
		return 0
	}
	return BreakevenSigma(feeAPR) / realizedVol
}

// Containment returns p = erf(k/sqrt2), the probability a standard normal lands
// within ±k. ≈0.6827 for k=1, ≈0.9545 for k=2. It is the containment confidence
// the ±kσ range band targets; it no longer feeds any edge calculation.
func Containment(k float64) float64 {
	if k <= 0 {
		return 0
	}
	return math.Erf(k / math.Sqrt2)
}

// ExpectedTimeInRangeDays returns the idealized mean number of days the centred
// log-price stays inside a ±kσ band before first touching an edge, under
// driftless GBM with instant re-centring.
//
// Under driftless GBM the expected first-passage time of the centred log-price
// to a band of log half-width delta is delta^2 / sigma^2 (in years, with sigma
// the annualised vol). Substituting the band's own width
// delta = k*sigma*sqrt(T/365) cancels sigma:
//
//	delta^2 / sigma^2 = k^2 * (T/365) years = k^2 * T days
//
// so
//
//	expected_time_in_range = k^2 * T   (days)
//
// It does NOT depend on the volatility (sigma cancels), precisely because the
// band is itself sized by sigma. This is INFORMATIONAL ONLY — it must not feed
// any edge or verdict. It is an idealized mean, not a guarantee: the real number
// of rebalances can be higher because the price can re-touch the edge several
// times.
func ExpectedTimeInRangeDays(k, horizonDays float64) float64 {
	if k <= 0 || horizonDays <= 0 {
		return 0
	}
	return k * k * horizonDays
}
