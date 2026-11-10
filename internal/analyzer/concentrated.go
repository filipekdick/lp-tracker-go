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

// DefaultHedgeDriftThreshold and DefaultHedgeExecFee are the default execution
// parameters used when evaluating a concentrated band's net viability with
// dynamic delta-hedging on perpetual futures.
const (
	DefaultHedgeDriftThreshold = 0.03   // 3% drift trigger
	DefaultHedgeExecFee        = 0.0003 // 3 bps blended maker/taker fee
	DefaultCrossCorrelation    = 0.80   // default ETH/BTC log-return correlation
)

// BandViability holds the concentrated net-edge and breakeven metrics for a
// specific ±delta range band once concentration C(delta), dynamic hedge
// rebalancing friction, and perp funding are accounted for.
type BandViability struct {
	Delta              float64 `json:"delta"`              // log half-width of the band
	ConcentrationX     float64 `json:"concentrationX"`     // C(delta) = 1 / (1 - e^(-delta/2))
	ProjectedAPR       float64 `json:"projectedApr"`       // projected position APR in the band
	LVRCostAPR         float64 `json:"lvrCostApr"`         // (sigma^2 / 8) * C(delta)
	HedgeFrictionAPR   float64 `json:"hedgeFrictionApr"`   // annualized perp rebalancing fee drag
	FundingAPR         float64 `json:"fundingApr"`         // annualized net perp funding (+ received, - paid)
	NetEdgeAPR         float64 `json:"netEdgeApr"`         // ProjectedAPR + FundingAPR - LVRCostAPR - HedgeFrictionAPR
	BreakevenSigma     float64 `json:"breakevenSigma"`     // max annualized vol this band's APR can sustain
	VolHeadroom        float64 `json:"volHeadroom"`        // BreakevenSigma / sigma (> 1 means viable)
	ExpectedDaysInBand float64 `json:"expectedDaysInBand"` // E[tau] = (delta / sigma)^2 * 365
}

// EvaluateBand computes the true mathematical viability of a concentrated LP
// position in a band of log half-width delta with dynamic delta hedging.
func EvaluateBand(
	projectedBandAPR, sigma, delta, driftThreshold, avgExecFee, fundingAPR float64,
) BandViability {
	if delta <= 0 || sigma <= 0 {
		return BandViability{}
	}
	c := ConcentrationFactor(delta)
	lvrCost := LVRCost(sigma) * c

	hedgeFrictionFactor := 0.10
	if driftThreshold > 0 && avgExecFee > 0 {
		hedgeFrictionFactor = math.Min(0.35, math.Max(0.03, (avgExecFee/driftThreshold)*12.0))
	}
	hedgeFriction := lvrCost * hedgeFrictionFactor

	effectiveYield := projectedBandAPR + fundingAPR
	netEdge := effectiveYield - lvrCost - hedgeFriction

	var breakevenSigma float64
	if effectiveYield > 0 {
		breakevenSigma = math.Sqrt(8.0 * effectiveYield / (c * (1.0 + hedgeFrictionFactor)))
	}

	return BandViability{
		Delta:              delta,
		ConcentrationX:     c,
		ProjectedAPR:       projectedBandAPR,
		LVRCostAPR:         lvrCost,
		HedgeFrictionAPR:   hedgeFriction,
		FundingAPR:         fundingAPR,
		NetEdgeAPR:         netEdge,
		BreakevenSigma:     breakevenSigma,
		VolHeadroom:        breakevenSigma / sigma,
		ExpectedDaysInBand: (delta / sigma) * (delta / sigma) * 365.0,
	}
}

// CrossImpliedVol computes the implied volatility of the ratio Token0/Token1
// (e.g. WETH/cbBTC) from the standalone USD implied volatilities of each leg
// and their correlation rho:
//
//	sigma_cross = sqrt(iv0^2 + iv1^2 - 2 * rho * iv0 * iv1)
//
// When rho is out of [-1, 1], DefaultCrossCorrelation (0.80) is used.
func CrossImpliedVol(iv0, iv1, rho float64) float64 {
	if iv0 <= 0 {
		return iv1
	}
	if iv1 <= 0 {
		return iv0
	}
	if rho < -1.0 || rho > 1.0 {
		rho = DefaultCrossCorrelation
	}
	variance := iv0*iv0 + iv1*iv1 - 2.0*rho*iv0*iv1
	if variance <= 0 {
		return 0
	}
	return math.Sqrt(variance)
}

// OptimalRangePlan describes the recommended range bounds and perp margin
// allocation for a concentrated LP + delta-hedge deployment.
type OptimalRangePlan struct {
	DeltaHalfWidth       float64 `json:"deltaHalfWidth"`       // log half-width delta
	LowerPct             float64 `json:"lowerPct"`             // e.g. -0.058 (-5.8%)
	UpperPct             float64 `json:"upperPct"`             // e.g. +0.062 (+6.2%)
	LowerPrice           float64 `json:"lowerPrice"`           // lower notional price
	UpperPrice           float64 `json:"upperPrice"`           // upper notional price
	ShortNotionalRatio   float64 `json:"shortNotionalRatio"`   // 0.50 for Token/Stable, 1.00 for Token/Token
	RecommendedMarginUSD float64 `json:"recommendedMarginUsd"` // perp margin to survive +maxAdversePumpPct
}

// BuildRangePlan sizes a [LowerPrice, UpperPrice] band from volatility sigma
// over horizonDays at confidence multiplier kSigma, and computes the recommended
// Binance futures margin buffer to avoid short liquidation during a pump.
func BuildRangePlan(
	spotPrice, sigma, horizonDays, kSigma, lpDepositUSD float64,
	bothLegsVolatile bool,
	maxAdversePumpPct float64,
) OptimalRangePlan {
	if sigma <= 0 || horizonDays <= 0 {
		return OptimalRangePlan{}
	}
	if kSigma <= 0 {
		kSigma = 1.0
	}
	if maxAdversePumpPct <= 0 {
		maxAdversePumpPct = 0.40
	}

	delta := kSigma * sigma * math.Sqrt(horizonDays/365.0)
	lowerPct := math.Exp(-delta) - 1.0
	upperPct := math.Exp(delta) - 1.0

	shortRatio := 0.50
	if bothLegsVolatile {
		shortRatio = 1.00
	}
	marginUSD := lpDepositUSD * shortRatio * (maxAdversePumpPct + 0.05)

	return OptimalRangePlan{
		DeltaHalfWidth:       delta,
		LowerPct:             lowerPct,
		UpperPct:             upperPct,
		LowerPrice:           spotPrice * math.Exp(-delta),
		UpperPrice:           spotPrice * math.Exp(delta),
		ShortNotionalRatio:   shortRatio,
		RecommendedMarginUSD: marginUSD,
	}
}
