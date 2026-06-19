package analyzer

// concentrated.go implements the Phase 3 generalized concentrated-LVR scan:
// evaluating a pool as if it were run as a concentrated (Uniswap-v3-style)
// position at an optimally chosen width, rather than only the full-range
// sigma^2/8 view.
//
// ---------------------------------------------------------------------------
// 1. Width parameter.
//
// We parameterise a range by its half-width in LOG price, delta:
//
//	price in [ p0 * e^{-delta} , p0 * e^{+delta} ]
//
// A log parameterisation is natural because volatility lives in log-price and
// because it makes the concentration factor below clean. We report the width to
// users as a percentage of the upper bound, WidthPct = e^{delta} - 1.
//
// 2. Concentration (capital-efficiency) factor C(delta).
//
// For a v3 position with liquidity L centred at price p0 over [p0 e^{-delta},
// p0 e^{+delta}], the position value (in quote units) works out to
//
//	V_conc = 2 * L * sqrt(p0) * (1 - e^{-delta/2})
//
// while a full-range (v2) position with the same L is worth V_full = 2 L
// sqrt(p0). The capital-efficiency factor is the ratio of the liquidity you get
// per dollar, i.e.
//
//	C(delta) = V_full / V_conc = 1 / (1 - e^{-delta/2})
//
// As delta -> infinity (full range) C -> 1; as delta -> 0 (very tight) C -> inf.
// A WIDER range has a SMALLER C. (Derivation: getAmount0/getAmount1 at the
// centre, see internal/v3math; matches Uniswap v3 whitepaper §6.)
//
// 3. Same-width invariant (the trap to avoid).
//
// While the price is in range, BOTH the fee yield and the LVR are produced by
// the SAME liquidity, so both are amplified by the SAME C(delta). The correct
// comparison is therefore amplified-fees vs amplified-LVR at one width — their
// ratio is independent of C. Comparing a concentrated cost against full-range
// fees (different widths) is the classic mistake and would make any tight range
// look free; we never do that.
//
// 4. Time in range and rebalancing.
//
// In log-price the centred price is a driftless Brownian motion with diffusion
// sigma (annualised). The expected time to first touch either edge of a
// symmetric band [-delta, +delta] is the classic result
//
//	E[tau] = delta^2 / sigma^2   (in years)
//
// so the position is re-centred at an expected rate of
//
//	rebalancesPerYear = sigma^2 / delta^2 .
//
// 5. Per-rebalance costs.
//
//   - rebalanceCost: gas + slippage to re-centre, a configurable fraction of
//     position value (default DefaultRebalanceCost).
//   - boundaryLoss(delta): the divergence (impermanent) loss realised when the
//     price is pushed to an edge and the position has become fully one-sided.
//     With s = e^{delta/2}, comparing LP value at the edge to holding the
//     centred inventory gives
//
//     boundaryLoss(delta) = s*(s-1) / (s^2 + 1) .
//
//     (Derivation in concentrated_test.go.) It grows with delta.
//
// 6. Expected net edge (annualised), as a function of width.
//
// Combining the four components the task calls for — amplified fee yield and
// amplified LVR weighted by time in range, minus per-cycle boundary loss and
// rebalancing cost amortised over the horizon — and using that with instant
// re-centring the fraction of time in range is ~1 (the cost of the brief
// excursions is captured by the per-rebalance terms), the per-year net edge is
//
//	NetEdge(delta) = (feeAPR - sigma^2/8) * C(delta)
//	                 - (sigma^2 / delta^2) * (rebalanceCost + boundaryLoss(delta))
//
// The first term pulls the width TIGHTER when fees beat LVR (bigger C), while the
// rebalancing term pulls it WIDER (fewer, cheaper re-centres). Their balance
// gives a single interior optimum, which we locate numerically.
//
// Modelling assumptions (documented for the report):
//   - GBM with zero drift in log-price (LVR's standard assumption).
//   - Instant, costless-of-time re-centring; excursions out of range are charged
//     only via boundaryLoss + rebalanceCost, not via lost fee time.
//   - boundaryLoss and continuous LVR both describe adverse selection and may
//     mildly overlap; the optimum and all direction invariants are unchanged
//     whether or not boundaryLoss is included (both it and the gas term push the
//     width wider), so we keep it for completeness and expose RebalanceCost to
//     tune the discrete-cost magnitude.

import "math"

// DefaultRebalanceCost is the default per-rebalance cost (gas + slippage) as a
// fraction of position value. 5 bps is a reasonable default for re-centring a
// modest position on an L2 like Base. Configurable per Input.
const DefaultRebalanceCost = 0.0005

// concentrated search bounds on the log half-width delta.
const (
	minDelta = 0.001 // ~±0.1%: tighter than any real LP would run
	maxDelta = 4.0   // C ≈ 1.03, effectively full range
)

// ConcentratedInput is the per-pool input to the concentrated-LVR optimiser.
type ConcentratedInput struct {
	FeeAPR        float64 // annualised fee yield on TVL (full-range)
	Sigma         float64 // annualised realised volatility
	HorizonDays   float64 // horizon T (informational; the optimum is per-year)
	RebalanceCost float64 // per-rebalance cost, fraction of value
}

// ConcentratedResult is the optimiser's output.
type ConcentratedResult struct {
	Delta             float64 `json:"delta"`             // optimal log half-width
	WidthPct          float64 `json:"widthPct"`          // e^delta - 1, the +range as a fraction
	ConcentrationC    float64 `json:"concentrationC"`    // C at the optimum
	NetEdgeAPR        float64 `json:"netEdgeApr"`        // expected net edge at the optimum
	RebalancesPerYear float64 `json:"rebalancesPerYear"` // sigma^2/delta^2 at the optimum
	FullRangeNetEdge  float64 `json:"fullRangeNetEdge"`  // feeAPR - sigma^2/8, for reference
}

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
// with s = e^{delta/2}.
func boundaryLoss(delta float64) float64 {
	if delta <= 0 {
		return 0
	}
	s := math.Exp(delta / 2.0)
	return s * (s - 1) / (s*s + 1)
}

// NetEdgeAtWidth evaluates the annualised expected net edge at log half-width
// delta. See the file header for the formula.
func NetEdgeAtWidth(in ConcentratedInput, delta float64) float64 {
	if delta <= 0 || in.Sigma <= 0 {
		return math.Inf(-1)
	}
	c := ConcentrationFactor(delta)
	a := in.FeeAPR - LVRCost(in.Sigma) // full-range net rate
	reb := (in.Sigma * in.Sigma) / (delta * delta)
	cost := in.RebalanceCost
	if cost < 0 {
		cost = 0
	}
	return a*c - reb*(cost+boundaryLoss(delta))
}

// OptimalConcentratedRange finds the width delta in [minDelta, maxDelta] that
// maximises NetEdgeAtWidth, via a coarse grid scan followed by a golden-section
// refinement around the best grid point. The grid scan makes the search robust
// even when the curve is monotone (then it returns a boundary).
func OptimalConcentratedRange(in ConcentratedInput) ConcentratedResult {
	res := ConcentratedResult{
		FullRangeNetEdge: in.FeeAPR - LVRCost(in.Sigma),
	}
	if in.Sigma <= 0 || in.FeeAPR <= 0 {
		// No meaningful trade-off; report a degenerate (full-range) optimum.
		res.Delta = maxDelta
		res.WidthPct = math.Exp(maxDelta) - 1
		res.ConcentrationC = ConcentrationFactor(maxDelta)
		res.NetEdgeAPR = res.FullRangeNetEdge
		return res
	}

	// Coarse grid in log-spaced delta for good resolution near tight ranges.
	const grid = 400
	bestDelta := minDelta
	bestVal := math.Inf(-1)
	lnLo, lnHi := math.Log(minDelta), math.Log(maxDelta)
	for i := 0; i <= grid; i++ {
		d := math.Exp(lnLo + (lnHi-lnLo)*float64(i)/float64(grid))
		if v := NetEdgeAtWidth(in, d); v > bestVal {
			bestVal, bestDelta = v, d
		}
	}

	// Golden-section refine within one grid cell either side of the best point.
	step := (lnHi - lnLo) / float64(grid)
	lo := math.Exp(math.Log(bestDelta) - step)
	hi := math.Exp(math.Log(bestDelta) + step)
	if lo < minDelta {
		lo = minDelta
	}
	if hi > maxDelta {
		hi = maxDelta
	}
	d := goldenMax(func(x float64) float64 { return NetEdgeAtWidth(in, x) }, lo, hi, 1e-7)

	res.Delta = d
	res.WidthPct = math.Exp(d) - 1
	res.ConcentrationC = ConcentrationFactor(d)
	res.NetEdgeAPR = NetEdgeAtWidth(in, d)
	res.RebalancesPerYear = (in.Sigma * in.Sigma) / (d * d)
	return res
}

// goldenMax returns the x in [lo,hi] maximising f, via golden-section search to
// the given absolute tolerance on x.
func goldenMax(f func(float64) float64, lo, hi, tol float64) float64 {
	const invphi = 0.6180339887498949 // 1/phi
	a, b := lo, hi
	c := b - invphi*(b-a)
	d := a + invphi*(b-a)
	fc, fd := f(c), f(d)
	for math.Abs(b-a) > tol {
		if fc > fd {
			b, d, fd = d, c, fc
			c = b - invphi*(b-a)
			fc = f(c)
		} else {
			a, c, fc = c, d, fd
			d = a + invphi*(b-a)
			fd = f(d)
		}
	}
	return (a + b) / 2
}
