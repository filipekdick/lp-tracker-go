package analyzer

// methods.go assembles the per-volatility-method view of a pool. Each method
// produces its own realised sigma and the downstream metrics recomputed from
// that sigma (LVR cost, net edge, fee/vol ratio, verdict), the Phase 2 range
// bands, and the expected net edge at the fixed ±1σ/±2σ bands.
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
	FeeImpliedVol float64 `json:"feeImpliedVol"` // sqrt(8*feeAPR); same across methods
	LVRCost       float64 `json:"lvrCost"`       // sigma^2/8 at this method's sigma
	NetEdgeAPR    float64 `json:"netEdgeApr"`    // feeAPR - LVRCost
	FeeYieldRatio float64 `json:"feeYieldRatio"` // feeAPR / LVRCost
	Verdict       Verdict `json:"verdict"`       // classification at this sigma

	// Phase 2: lognormal containment bands at HorizonDays, as up/down fractions.
	HorizonDays float64 `json:"horizonDays"`
	Band1Up     float64 `json:"band1Up"`   // +1σ up move (fraction, e.g. 0.05)
	Band1Down   float64 `json:"band1Down"` // -1σ down move (positive magnitude)
	Band2Up     float64 `json:"band2Up"`   // +2σ up move
	Band2Down   float64 `json:"band2Down"` // -2σ down move (positive magnitude)

	// Expected net edge at the fixed ±k-sigma band (k=1 and k=2), the
	// time-in-range-weighted estimate that replaced the corner-prone optimum.
	ExpEdge1 BandEdgeResult `json:"expEdge1"`
	ExpEdge2 BandEdgeResult `json:"expEdge2"`
}

// buildMethods computes every selectable method from the supplied bars. feeAPR
// is computed once by Analyze and shared, since it does not depend on the
// volatility method.
func buildMethods(in Input, feeAPR float64) map[string]MethodResult {
	feeImplied := FeeImpliedVolatility(feeAPR)
	horizon := in.HorizonDays
	if horizon <= 0 {
		horizon = DefaultHorizonDays
	}
	rebalCost := in.RebalanceCost
	if rebalCost <= 0 {
		rebalCost = DefaultRebalanceCost
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
		}
		if ok {
			mr.LVRCost = LVRCost(sigma)
			mr.NetEdgeAPR = feeAPR - mr.LVRCost
			if mr.LVRCost > 0 {
				mr.FeeYieldRatio = feeAPR / mr.LVRCost
			}
			mr.Verdict = classifyRatio(feeAPR, mr.FeeYieldRatio, sigma)

			// Phase 2: lognormal containment bands at the horizon.
			b1 := LognormalBands(sigma, horizon, 1)
			b2 := LognormalBands(sigma, horizon, 2)
			mr.Band1Up, mr.Band1Down = b1.UpPct, b1.DownPct
			mr.Band2Up, mr.Band2Down = b2.UpPct, b2.DownPct

			// Expected net edge at the fixed ±1σ and ±2σ bands, with fee capture
			// weighted by time-in-range (containment proxy). Bounded, comparable.
			mr.ExpEdge1 = ExpectedBandEdge(BandEdgeInput{FeeAPR: feeAPR, Sigma: sigma, K: 1, HorizonDays: horizon, RebalanceCost: rebalCost})
			mr.ExpEdge2 = ExpectedBandEdge(BandEdgeInput{FeeAPR: feeAPR, Sigma: sigma, K: 2, HorizonDays: horizon, RebalanceCost: rebalCost})
		} else {
			mr.Verdict = VerdictUnknown
		}
		out[string(m)] = mr
	}
	return out
}
