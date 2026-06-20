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
	LowerThreshold float64 // e.g., 0.03 for 3%
	UpperThreshold float64 // e.g., 0.06 for 6%
}

// Decide evaluates the drift percentage (absolute drift tokens / target short tokens)
// and returns the appropriate hedging Action and a descriptive reason.
func (s *Strategy) Decide(driftPercentage float64) (Action, string) {
	absDrift := math.Abs(driftPercentage)

	if absDrift < s.LowerThreshold {
		return ActionLimit, "drift < lower threshold"
	}
	if absDrift >= s.UpperThreshold {
		return ActionMarket, "drift >= upper threshold"
	}
	return ActionNone, "drift in dead band, holding"
}
