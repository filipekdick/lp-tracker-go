package analyzer

// methods.go assembles the per-volatility-method view of a pool. Each method
// produces its own realised sigma and the downstream metrics recomputed from
// that sigma (LVR cost, full-range net edge, fee/vol ratio, verdict), the Phase 2
// range bands (range-sizing guidance only), the volatility headroom, and the
// informational expected time in range.
//
// All methods are derived from the single OHLCV slice in Input.Bars, so the
// whole map costs one API request per pool.

// MethodResult is one volatility method's full analysis. Volatilities and rates
// are annualised fractions (0.5 == 50%).
type MethodResult struct {
	Method string `json:"method"` // VolMethod key
	Label  string `json:"label"`  // human-readable label
	OK     bool   `json:"ok"`     // false == not enough data for this method

	RealizedVol   float64 `json:"realizedVol"`   // annualised realised volatility
	FeeImpliedVol float64 `json:"feeImpliedVol"` // sqrt(8*feeAPR); also the breakeven sigma, same across methods
	LVRCost       float64 `json:"lvrCost"`       // sigma^2/8 at this method's sigma
	NetEdgeAPR    float64 `json:"netEdgeApr"`    // feeAPR - LVRCost (full-range net edge)
	FeeYieldRatio float64 `json:"feeYieldRatio"` // feeAPR / LVRCost
	Verdict       Verdict `json:"verdict"`       // classification at this sigma

	// Phase 2: lognormal containment bands at HorizonDays, as up/down fractions.
	// These are pure range-sizing guidance: where to set the LP to stay in range
	// ~68% (k=1) / ~95% (k=2) of the time over the horizon — NOT an edge or return.
	HorizonDays float64 `json:"horizonDays"`
	Band1Up     float64 `json:"band1Up"`   // +1σ up move (fraction, e.g. 0.05)
	Band1Down   float64 `json:"band1Down"` // -1σ down move (positive magnitude)
	Band2Up     float64 `json:"band2Up"`   // +2σ up move
	Band2Down   float64 `json:"band2Down"` // -2σ down move (positive magnitude)

	// VolHeadroom = breakeven (fee-implied) sigma / realized sigma = the
	// attractiveness ratio in volatility space. Above 1 means fees overpay for the
	// realized vol; it is the square root of the old fee/vol APR ratio.
	VolHeadroom float64 `json:"volHeadroom"`

	// Expected time in range (days), informational only: k^2 * T at this method's
	// horizon, for k=1 and k=2. Independent of sigma (the band is sized by sigma).
	// Displayed alongside the range bands; does NOT feed any edge or verdict.
	ExpTimeInRange1 float64 `json:"expTimeInRange1"`
	ExpTimeInRange2 float64 `json:"expTimeInRange2"`
}

// buildMethods computes every selectable method from the supplied bars. feeAPR
// is computed once by Analyze and shared, since it does not depend on the
// volatility method.
func buildMethods(in Input, feeAPR float64) map[string]MethodResult {
	feeImplied := FeeImpliedVolatility(feeAPR) // == BreakevenSigma(feeAPR)
	horizon := in.HorizonDays
	if horizon <= 0 {
		horizon = DefaultHorizonDays
	}

	out := make(map[string]MethodResult, len(AllMethods()))
	for _, m := range AllMethods() {
		sigma, ok := RealizedVolByMethod(in.Bars, in.PeriodsPerYear, m)
		mr := MethodResult{
			Method:        string(m),
			Label:         MethodLabel(m),
			OK:            ok,
			RealizedVol:   sigma,
			FeeImpliedVol: feeImplied,
			HorizonDays:   horizon,
			// Expected time in range = k^2*T days; sigma cancels (band is sized by
			// sigma), so it is well-defined even when a method has no usable sigma.
			ExpTimeInRange1: ExpectedTimeInRangeDays(1, horizon),
			ExpTimeInRange2: ExpectedTimeInRangeDays(2, horizon),
		}
		if ok {
			mr.LVRCost = LVRCost(sigma)
			mr.NetEdgeAPR = feeAPR - mr.LVRCost
			if mr.LVRCost > 0 {
				mr.FeeYieldRatio = feeAPR / mr.LVRCost
			}
			mr.Verdict = classifyRatio(feeAPR, mr.FeeYieldRatio, sigma)

			// Phase 2: lognormal containment bands at the horizon — range-sizing
			// guidance only (stay in range ~68%/~95% of the time over T).
			b1 := LognormalBands(sigma, horizon, 1)
			b2 := LognormalBands(sigma, horizon, 2)
			mr.Band1Up, mr.Band1Down = b1.UpPct, b1.DownPct
			mr.Band2Up, mr.Band2Down = b2.UpPct, b2.DownPct

			// Volatility headroom = breakeven sigma / realized sigma. The honest,
			// pool-data-only attractiveness ratio in volatility space.
			mr.VolHeadroom = VolHeadroom(feeAPR, sigma)
		} else {
			mr.Verdict = VerdictUnknown
		}
		out[string(m)] = mr
	}
	return out
}
