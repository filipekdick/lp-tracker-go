package hedger

import (
	"math"
)

// Action represents the chosen hedging order type.
type Action string

const (
	ActionLimit  Action = "LIMIT"
	ActionMarket Action = "MARKET"
	ActionNone   Action = "NONE"
)

// Strategy determines the hedging action based on drift percentages.
type Strategy struct {
	LowerThreshold       float64 // e.g., 0.03 for 3%: below this is the dead band (ActionNone)
	UpperThreshold       float64 // e.g., 0.06 for 6%: [Lower, Upper) uses ActionLimit; >= Upper uses ActionMarket
	EdgeBufferMultiplier float64 // e.g., 1.5x threshold multiplier near range boundaries to prevent whipsaw
}

// Decide evaluates the drift percentage (absolute drift tokens / target short tokens)
// and returns the appropriate hedging Action and a descriptive reason:
//   - |drift| < LowerThreshold               -> ActionNone   (dead band, holding)
//   - LowerThreshold <= |drift| < UpperThreshold -> ActionLimit  (passive maker limit order)
//   - |drift| >= UpperThreshold              -> ActionMarket (urgent market rebalance)
func (s *Strategy) Decide(driftPercentage float64) (Action, string) {
	return s.DecideAtRange(driftPercentage, 50.0)
}

// DecideAtRange applies extra hysteresis when the position price sits near the
// lower or upper tick boundaries (rangePositionPct <= 5% or >= 95%) to avoid
// costly whipsaw rebalancing right at the range edge.
func (s *Strategy) DecideAtRange(driftPercentage, rangePositionPct float64) (Action, string) {
	absDrift := math.Abs(driftPercentage)
	lower := s.LowerThreshold
	upper := s.UpperThreshold

	if rangePositionPct <= 5.0 || rangePositionPct >= 95.0 {
		mult := s.EdgeBufferMultiplier
		if mult <= 1.0 {
			mult = 1.5
		}
		lower *= mult
		upper *= mult
	}

	if absDrift < lower {
		return ActionNone, "drift in dead band (< lower threshold), holding"
	}
	if absDrift < upper {
		return ActionLimit, "drift in passive band (lower <= drift < upper), placing maker limit"
	}
	return ActionMarket, "drift >= upper threshold, executing market rebalance"
}
