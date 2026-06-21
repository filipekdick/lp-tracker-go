package analyzer

// concentrated.go evaluates a pool as a concentrated (Uniswap-v3-style) position
// at a FIXED, principled band — the ±k-sigma containment range from the Phase 2
// work — and reports the expected net edge there.
//
// Why a fixed band rather than an optimised width. An earlier version searched
// for the profit-maximising width, but the model amplifies fees by the
// concentration factor C without penalising a tight band with lost fees, so the
// search always pinned the width at the grid floor (absurd idealised edge) or
// ran to the ceiling — a useless corner solution. The fix, below, weights the
// amplified fee capture by the fraction of time the position is actually in
// range, so a tighter band's larger C is offset by a smaller in-range fraction.
//
// ---------------------------------------------------------------------------
// Band. We reuse the ±k-sigma lognormal band (Phase 2). Its log half-width is
//
//	delta = k * sigma * sqrt(T / 365)
//
// with T the horizon in days and k ∈ {1, 2}. Because delta scales with sigma,
// the containment probability is (by construction) ~constant in sigma:
//
//	p = P(|N(0,1)| <= k) = erf(k / sqrt(2))   (≈0.6827 for k=1, ≈0.9545 for k=2)
//
// We use p as a proxy for the fraction of time the price is in range (the price
// is within ±k-sigma of centre with probability p at the horizon). This is a
// containment proxy, not a guaranteed return.
//
// Concentration factor: C(delta) = 1 / (1 - e^{-delta/2}).
//
// Expected net edge (annualised), at the fixed band:
//
//	expectedFee = feeAPR              * p   // pool yield, weighted by time in range — NO C
//	expectedLVR = C(delta) * (sigma^2/8) * p   // amplified divergence cost — C belongs here
//	exitCost    = (sigma^2 / delta^2) * (rebalanceCost + boundaryLoss(delta))
//	netEdge     = expectedFee - expectedLVR - exitCost
//
// IMPORTANT — the fee term has NO concentration factor. feeAPR is already the
// yield earned by the pool's existing (concentrated) deposited liquidity:
// feeAPR = volume*feeTier / TVL, and TVL is the pool's actual, already-small
// concentrated capital. Multiplying it by C again double-counts concentration —
// the bug this corrects. The effect was worst for low-vol pairs (tiny sigma ->
// tiny band -> huge C -> blown-up "edge"). Concentration legitimately appears
// only on the impermanent-loss / LVR term (a concentrated position suffers
// amplified divergence) and, implicitly, in the exit term via the band width.
//
// Boundedness invariant (by construction): expectedLVR >= 0 and exitCost >= 0, so
//
//	netEdge <= feeAPR * p <= feeAPR
//
// The exit term is the expected number of boundary exits per year (the BM
// first-exit rate sigma^2/delta^2) times the per-exit cost (gas to re-centre
// plus the realised divergence loss at the edge). Substituting delta gives
// sigma^2/delta^2 = 365 / (k^2 * T), i.e. ~once per day for a ±1σ daily band.
//
// Breakeven volatility. A clean, bounded way to read the same result is the
// concentration-aware breakeven sigma — the realised vol at which fees exactly
// equal the amplified IL (expectedFee == expectedLVR):
//
//	sigma_star = sqrt( 8 * feeAPR / C(delta) )
//
// using the EXACT C(delta). At full range (delta -> inf, C -> 1) this reduces to
// the existing fee-implied volatility sqrt(8*feeAPR), so it is a strict
// generalisation of that metric. If realised sigma < sigma_star the band is
// profitable, and sigma_star / sigma is the headroom.
//
// Modelling assumptions (for the report):
//   - GBM with zero drift in log-price (LVR's standard assumption).
//   - Terminal containment p used as a time-in-range proxy.
//   - feeAPR is the pool's quoted yield at its current concentration; we do NOT
//     re-amplify it, so the estimate is bounded above by feeAPR.

import "math"

// DefaultRebalanceCost is the default per-rebalance cost (gas + slippage) as a
// fraction of position value. 5 bps is a reasonable default for re-centring a
// modest position on an L2 like Base. Configurable per Input.
const DefaultRebalanceCost = 0.0005

// ConcentrationFactor returns C(delta) = 1/(1 - e^{-delta/2}).
func ConcentrationFactor(delta float64) float64 {
	if delta <= 0 {
		return math.Inf(1)
	}
	return 1.0 / (1.0 - math.Exp(-delta/2.0))
}

// AmplifiedFeeYield returns the in-range fee yield of a concentrated position at
// width delta: feeAPR * C(delta).
func AmplifiedFeeYield(feeAPR, delta float64) float64 {
	return feeAPR * ConcentrationFactor(delta)
}

// AmplifiedLVR returns the in-range LVR of a concentrated position at width
// delta: (sigma^2/8) * C(delta). Both fees and LVR use the SAME C.
func AmplifiedLVR(sigma, delta float64) float64 {
	return LVRCost(sigma) * ConcentrationFactor(delta)
}

// boundaryLoss returns the divergence loss realised at an edge, s(s-1)/(s^2+1)
// with s = e^{delta/2}. Derivation cross-checked in concentrated_test.go.
func boundaryLoss(delta float64) float64 {
	if delta <= 0 {
		return 0
	}
	s := math.Exp(delta / 2.0)
	return s * (s - 1) / (s*s + 1)
}

// BreakevenSigma returns the concentration-aware breakeven volatility for a band
// of log half-width delta: sigma* = sqrt(8 * feeAPR / C(delta)), the realised vol
// at which fee income exactly equals the amplified IL. Uses the EXACT C(delta);
// at full range (C -> 1) it reduces to the fee-implied vol sqrt(8*feeAPR).
func BreakevenSigma(feeAPR, delta float64) float64 {
	if feeAPR <= 0 {
		return 0
	}
	c := ConcentrationFactor(delta)
	if c <= 0 || math.IsInf(c, 0) {
		return 0 // degenerate (zero-width band): breaks even only at zero vol
	}
	return math.Sqrt(8 * feeAPR / c)
}

// Containment returns p = erf(k/sqrt2), the probability a standard normal lands
// within ±k. ≈0.6827 for k=1, ≈0.9545 for k=2.
func Containment(k float64) float64 {
	if k <= 0 {
		return 0
	}
	return math.Erf(k / math.Sqrt2)
}

// BandEdgeInput is the input to the fixed-band expected-edge estimate.
type BandEdgeInput struct {
	FeeAPR        float64 // annualised fee yield on TVL (full-range)
	Sigma         float64 // annualised realised volatility
	K             float64 // band width in sigmas (1 or 2)
	HorizonDays   float64 // horizon T in days
	RebalanceCost float64 // gas per re-centre, fraction of value
}

// BandEdgeResult holds the expected net edge at the ±k-sigma band and its parts.
type BandEdgeResult struct {
	K              float64 `json:"k"`              // band width in sigmas
	Delta          float64 `json:"delta"`          // log half-width = k*sigma*sqrt(T/365)
	WidthPct       float64 `json:"widthPct"`       // e^delta - 1, the +range as a fraction
	Containment    float64 `json:"containment"`    // p, the time-in-range proxy
	ConcentrationC float64 `json:"concentrationC"` // C(delta)
	ExpectedFeeAPR float64 `json:"expectedFeeApr"` // feeAPR*p (NO concentration factor)
	ExpectedLVRAPR float64 `json:"expectedLvrApr"` // C*(sigma^2/8)*p
	ExitCostAPR    float64 `json:"exitCostApr"`    // (sigma^2/delta^2)*(gas+boundaryLoss)
	NetEdgeAPR     float64 `json:"netEdgeApr"`     // expectedFee - expectedLVR - exitCost (<= feeAPR)
	SigmaStar      float64 `json:"sigmaStar"`      // breakeven vol sqrt(8*feeAPR/C(delta))
}

// ExpectedBandEdge computes the expected net edge of running the pool as a
// concentrated position at the ±k-sigma band over the horizon. See the file
// header for the assembly and assumptions.
func ExpectedBandEdge(in BandEdgeInput) BandEdgeResult {
	k := in.K
	if k <= 0 {
		k = 1
	}
	T := in.HorizonDays
	if T <= 0 {
		T = DefaultHorizonDays
	}
	gas := in.RebalanceCost
	if gas < 0 {
		gas = 0
	}
	p := Containment(k)
	res := BandEdgeResult{K: k, Containment: p}

	// Degenerate: a flat series has no width and no LVR; a position never exits,
	// so it captures fees with no exit cost. Report the full-range (C=1) breakeven.
	if in.Sigma <= 0 {
		res.ConcentrationC = 1
		res.ExpectedFeeAPR = in.FeeAPR * p
		res.NetEdgeAPR = res.ExpectedFeeAPR
		res.SigmaStar = FeeImpliedVolatility(in.FeeAPR)
		return res
	}

	delta := k * in.Sigma * math.Sqrt(T/365.0)
	c := ConcentrationFactor(delta)

	res.Delta = delta
	res.WidthPct = math.Exp(delta) - 1
	res.ConcentrationC = c
	// Fee term is the pool's quoted yield weighted by time in range — NO C, since
	// feeAPR already reflects the pool's existing concentration (see file header).
	res.ExpectedFeeAPR = in.FeeAPR * p
	// Concentration amplifies only the impermanent-loss / LVR term.
	res.ExpectedLVRAPR = c * LVRCost(in.Sigma) * p
	res.ExitCostAPR = (in.Sigma * in.Sigma) / (delta * delta) * (gas + boundaryLoss(delta))
	res.NetEdgeAPR = res.ExpectedFeeAPR - res.ExpectedLVRAPR - res.ExitCostAPR
	res.SigmaStar = BreakevenSigma(in.FeeAPR, delta)
	return res
}
