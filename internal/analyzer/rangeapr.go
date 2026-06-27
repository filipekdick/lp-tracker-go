package analyzer

// rangeapr.go projects the fee APR of a concentrated-liquidity position the way
// the official LP aggregators (Revert, Krystal, Uniswap's own position UI) do:
// by the share of the IN-RANGE liquidity a deposit would capture.
//
// The model. Swap fees accrue to whoever provides the active liquidity at the
// price where the swap happens. For a range that contains the current price, the
// trailing swap fees are
//
//	annualFees = volume24h * feeTier * 365
//
// and a deposit of V competing against L_active of already-active liquidity in
// that range earns the fraction V / (L_active + V) of them:
//
//	projectedAPR = annualFees * V / (L_active + V) / V
//	             = volume24h * feeTier * 365 / (L_active + V)
//
// As V -> 0 this is the MARGINAL apr (what one extra dollar would earn); for a
// non-trivial V it self-dilutes, exactly as a real deposit dilutes the pool.
//
// Estimating L_active from a range. Concentrating into a narrower range puts the
// same dollar to work as MORE liquidity at the current price (capital
// efficiency), so it competes against a SMALLER slice of the pool's TVL. We use
// the pool's own concentration factor C(delta) = 1/(1 - e^{-delta/2}) for a band
// of log half-width delta (the same C used throughout concentrated.go) as the
// bridge:
//
//	L_active = TVL / C(delta)
//
// so the marginal APR is fullRangeAPR * C(delta) — the familiar capital-
// efficiency boost — while still diluting correctly for a real deposit. As the
// range widens (delta large) C -> 1 and L_active -> TVL, recovering the
// full-range APR. This needs no on-chain tick data, only the pool's TVL, volume,
// fee tier and a chosen range.
//
// CAVEAT — like BreakevenSigma this is OPTIMISTIC: it assumes the pool's own
// liquidity is no more concentrated than full-range, and it counts only swap
// fees (no farm/gauge incentives). It is a projection, not a guarantee.

import "math"

// RangeSimulation is the projected outcome of depositing depositUSD into a
// concentrated price range, in the shape an LP aggregator would present.
type RangeSimulation struct {
	DepositUSD         float64 `json:"depositUsd"`         // simulated deposit size
	LowerPrice         float64 `json:"lowerPrice"`         // range lower bound (price)
	UpperPrice         float64 `json:"upperPrice"`         // range upper bound (price)
	WidthPct           float64 `json:"widthPct"`           // ± fraction around the current price (0 if priced directly)
	ConcentrationX     float64 `json:"concentrationX"`     // capital-efficiency multiplier vs full range
	ActiveLiquidityUSD float64 `json:"activeLiquidityUsd"` // estimated competing in-range liquidity
	FeeAPR             float64 `json:"feeApr"`             // projected fee APR for the deposit
	DailyFeesUSD       float64 `json:"dailyFeesUsd"`       // projected fees per day for the deposit
}

// RangeFeeAPR is the aggregator share-of-liquidity fee APR for a deposit of
// depositUSD into a range whose currently-active competing liquidity is
// activeLiquidityUSD:
//
//	APR = volume24h * feeTier * 365 / (activeLiquidityUSD + depositUSD)
//
// depositUSD may be 0 to get the marginal (one-extra-dollar) APR. Returns 0 when
// there is no fee flow or no liquidity to compete against.
func RangeFeeAPR(volume24hUSD, feeTier, depositUSD, activeLiquidityUSD float64) float64 {
	denom := activeLiquidityUSD + depositUSD
	if volume24hUSD <= 0 || feeTier <= 0 || denom <= 0 {
		return 0
	}
	return volume24hUSD * feeTier * 365.0 / denom
}

// logHalfWidthFromPrices returns the log half-width delta of a price range
// [lower, upper], i.e. (ln upper - ln lower)/2. Zero for a degenerate range.
func logHalfWidthFromPrices(lower, upper float64) float64 {
	if lower <= 0 || upper <= lower {
		return 0
	}
	return math.Log(upper/lower) / 2.0
}

// ActiveLiquidityInRangeUSD estimates the USD liquidity already competing for
// fees inside a price range, as TVL / C(delta) where delta is the range's log
// half-width (see file header). A wider range competes against more of the
// pool's TVL; a tighter range against less. A degenerate or full range returns
// the whole TVL.
func ActiveLiquidityInRangeUSD(tvlUSD, lowerPrice, upperPrice float64) float64 {
	if tvlUSD <= 0 {
		return 0
	}
	delta := logHalfWidthFromPrices(lowerPrice, upperPrice)
	c := ConcentrationFactor(delta)
	if math.IsInf(c, 1) || c <= 1 {
		return tvlUSD
	}
	return tvlUSD / c
}

// SimulateRangeAPRPrices projects the fee APR for a deposit into an explicit
// price range [lowerPrice, upperPrice], using the pool's volume, fee tier and
// TVL. It is the variant used when concrete range prices are known (e.g. a
// tracked position's own tick range).
func SimulateRangeAPRPrices(volume24hUSD, feeTier, tvlUSD, currentPrice, lowerPrice, upperPrice, depositUSD float64) RangeSimulation {
	delta := logHalfWidthFromPrices(lowerPrice, upperPrice)
	active := ActiveLiquidityInRangeUSD(tvlUSD, lowerPrice, upperPrice)
	apr := RangeFeeAPR(volume24hUSD, feeTier, depositUSD, active)
	sim := RangeSimulation{
		DepositUSD:         depositUSD,
		LowerPrice:         lowerPrice,
		UpperPrice:         upperPrice,
		ConcentrationX:     ConcentrationFactor(delta),
		ActiveLiquidityUSD: active,
		FeeAPR:             apr,
		DailyFeesUSD:       apr * depositUSD / 365.0,
	}
	if currentPrice > 0 {
		sim.WidthPct = (upperPrice - lowerPrice) / (2 * currentPrice)
	}
	return sim
}

// SimulateRangeAPR projects the fee APR for a deposit into a range of half-width
// widthPct around the current price (e.g. widthPct 0.1 == a ±10% range). It is
// the variant used to scan/simulate hypothetical positions when only a width is
// chosen. widthPct must be in (0, 1).
func SimulateRangeAPR(volume24hUSD, feeTier, tvlUSD, currentPrice, widthPct, depositUSD float64) RangeSimulation {
	if currentPrice <= 0 || widthPct <= 0 || widthPct >= 1 {
		return RangeSimulation{DepositUSD: depositUSD, WidthPct: widthPct}
	}
	lower := currentPrice * (1 - widthPct)
	upper := currentPrice * (1 + widthPct)
	sim := SimulateRangeAPRPrices(volume24hUSD, feeTier, tvlUSD, currentPrice, lower, upper, depositUSD)
	sim.WidthPct = widthPct
	return sim
}

// SimulateVolRange projects the fee APR for a deposit into a range sized by the
// pool's own realised volatility — a ±z-sigma containment band over horizonDays
// (the same band geometry as ranges.go). This is the "search liquidity in a
// range" default: the range adapts to each pool's volatility rather than a fixed
// width, and the APR reflects the concentration that range affords. Returns a
// zero simulation when there is no usable volatility signal.
func SimulateVolRange(volume24hUSD, feeTier, tvlUSD, currentPrice, realizedVol, horizonDays, z, depositUSD float64) RangeSimulation {
	if horizonDays <= 0 {
		horizonDays = DefaultHorizonDays
	}
	if z <= 0 {
		z = 1
	}
	delta := LogHalfWidth(realizedVol, horizonDays, z)
	if delta <= 0 || currentPrice <= 0 {
		return RangeSimulation{DepositUSD: depositUSD}
	}
	lower := currentPrice * math.Exp(-delta)
	upper := currentPrice * math.Exp(delta)
	return SimulateRangeAPRPrices(volume24hUSD, feeTier, tvlUSD, currentPrice, lower, upper, depositUSD)
}
