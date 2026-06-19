package analyzer

// ranges.go implements the Phase 2 "ideal range" statistical bands.
//
// IMPORTANT: these bands are a CONTAINMENT target — the price-move that the
// realised volatility says will (with ~68% / ~95% confidence) not be exceeded
// over the horizon. They are NOT a profit-optimal liquidity width. The
// profit-optimal width, which trades fee amplification against rebalancing
// cost, is computed separately in concentrated.go (Phase 3).
//
// Model. Under geometric Brownian motion the log-price over a horizon of T days
// is Normally distributed with standard deviation
//
//	s = sigma * sqrt(T / 365)
//
// where sigma is the annualised volatility (variance grows linearly in time, so
// volatility grows with the square root of time). A z-sigma move in log-price is
// therefore z*s, and because price = exp(log-price) the bounds are
// multiplicative (lognormal):
//
//	up   = exp(+z*s) - 1      (price rises by this fraction)
//	down = 1 - exp(-z*s)      (price falls by this fraction, magnitude)
//
// The up move is strictly larger in magnitude than the down move (exp(x)-1 >
// 1-exp(-x) for x>0): a +z and -z move in log-space are symmetric, but in price
// terms the upside compounds while the downside is bounded by zero.

import "math"

// DefaultHorizonDays is the default range/optimiser horizon T. We default to one
// day: it is the natural cadence at which a concentrated LP would re-check and
// potentially re-centre a position, and it keeps the bands legible (the pool
// scan interval itself is only an informational refresh cadence, far too short
// to be a meaningful price-range horizon). It is configurable per Input.
const DefaultHorizonDays = 1.0

// RangeBands holds a symmetric-in-log, asymmetric-in-price containment band as
// up/down fractions (e.g. UpPct 0.05 == +5%). DownPct is a positive magnitude.
type RangeBands struct {
	UpPct   float64 `json:"upPct"`
	DownPct float64 `json:"downPct"`
}

// LogHalfWidth returns the half-width of the band in log-price space, z*s, where
// s = sigma*sqrt(T/365). This is the quantity that scales exactly with the
// square root of time: doubling T multiplies it by sqrt(2).
func LogHalfWidth(sigma, horizonDays, z float64) float64 {
	if sigma <= 0 || horizonDays <= 0 || z <= 0 {
		return 0
	}
	return z * sigma * math.Sqrt(horizonDays/365.0)
}

// LognormalBands returns the z-sigma containment band over the horizon as
// up/down price fractions.
func LognormalBands(sigma, horizonDays, z float64) RangeBands {
	s := LogHalfWidth(sigma, horizonDays, z)
	if s <= 0 {
		return RangeBands{}
	}
	return RangeBands{
		UpPct:   math.Exp(s) - 1,
		DownPct: 1 - math.Exp(-s),
	}
}
