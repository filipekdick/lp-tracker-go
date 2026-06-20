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
// Concentration factor (unchanged): C(delta) = 1 / (1 - e^{-delta/2}); while in
// range BOTH fees and LVR are amplified by the same C.
//
// Expected net edge (annualised), at the fixed band:
//
//	expectedFee = C(delta) * feeAPR     * p        // amplified fees, weighted by time in range
//	expectedLVR = C(delta) * (sigma^2/8) * p       // amplified LVR, weighted the same way
//	exitCost    = (sigma^2 / delta^2) * (rebalanceCost + boundaryLoss(delta))
//	netEdge     = expectedFee - expectedLVR - exitCost
//
// The exit term is the expected number of boundary exits per year (the BM
// first-exit rate sigma^2/delta^2) times the per-exit cost (gas to re-centre
// plus the realised divergence loss at the edge). Substituting delta gives
// sigma^2/delta^2 = 365 / (k^2 * T), i.e. ~once per day for a ±1σ daily band, so
// the whole expression is bounded and comparable across pools.
//
// Modelling assumptions (for the report):
//   - GBM with zero drift in log-price (LVR's standard assumption).
//   - Terminal containment p used as a time-in-range proxy.
//   - feeAPR is the pool's current yield, taken independent of position size, so
//     a very-low-vol / high-fee pool still shows a large (but finite, non-corner)
//     idealised edge — an upper bound, not a guaranteed return.

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
	ExpectedFeeAPR float64 `json:"expectedFeeApr"` // C*feeAPR*p
	ExpectedLVRAPR float64 `json:"expectedLvrApr"` // C*(sigma^2/8)*p
	ExitCostAPR    float64 `json:"exitCostApr"`    // (sigma^2/delta^2)*(gas+boundaryLoss)
	NetEdgeAPR     float64 `json:"netEdgeApr"`     // expectedFee - expectedLVR - exitCost
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
	// so it captures fees with no amplification claim and no exit cost.
	if in.Sigma <= 0 {
		res.ConcentrationC = 1
		res.ExpectedFeeAPR = in.FeeAPR * p
		res.NetEdgeAPR = res.ExpectedFeeAPR
		return res
	}

	delta := k * in.Sigma * math.Sqrt(T/365.0)
	c := ConcentrationFactor(delta)

	res.Delta = delta
	res.WidthPct = math.Exp(delta) - 1
	res.ConcentrationC = c
	res.ExpectedFeeAPR = c * in.FeeAPR * p
	res.ExpectedLVRAPR = c * LVRCost(in.Sigma) * p
	res.ExitCostAPR = (in.Sigma * in.Sigma) / (delta * delta) * (gas + boundaryLoss(delta))
	res.NetEdgeAPR = res.ExpectedFeeAPR - res.ExpectedLVRAPR - res.ExitCostAPR
	return res
}
