package position

// This file holds the two cumulative ledgers that keep the strategy PnL honest:
//
//   1. The hedge PnL identity — combining the open position's unrealized PnL
//      with the realized PnL banked at each rebalance, plus funding and
//      commissions re-derived from Binance income history.
//   2. The LP fee ledger — banking collected fees on a harvest so the cumulative
//      fee total (collected + uncollected) never collapses when the position is
//      harvested.
//
// Both follow the same realized+unrealized pattern: a "banked" part that only
// grows and a "live" part that resets, whose sum is continuous across the event
// that moves value from live to banked.

// CompleteHedgePnL is the full hedge PnL: cumulative realized PnL (closed legs)
// plus current unrealized PnL (open position). Realized and unrealized are
// DISJOINT — a leg's PnL is unrealized while the position is open and becomes
// realized only once it is closed, at which point it leaves the unrealized
// figure — so the two add without double-counting.
func CompleteHedgePnL(realized, unrealized float64) float64 {
	return realized + unrealized
}

// PnLComponents are the signed terms of the strategy net PnL, each measured
// since strategy inception. NetPnL sums them with the documented signs.
type PnLComponents struct {
	// LPChange is the change in LP position value since inception (+ = gain).
	LPChange float64
	// HedgeUnrealized is the open short's current unrealized PnL, relative to
	// inception (+ = the short is in profit).
	HedgeUnrealized float64
	// HedgeRealized is the cumulative realized PnL of closed hedge legs (+ = gain).
	HedgeRealized float64
	// Funding is the cumulative funding, signed (+ = the short received funding).
	Funding float64
	// Commissions is the cumulative trading commission PAID, a positive cost that
	// is SUBTRACTED.
	Commissions float64
	// LPFees is the cumulative LP fees since inception (collected + uncollected),
	// added (+ = fees earned).
	LPFees float64
}

// NetPnL is the strategy net PnL identity:
//
//	net PnL = LP value change since inception
//	        + hedge unrealized
//	        + hedge realized
//	        + funding              (signed)
//	        - commissions          (a paid cost)
//	        + LP fees              (cumulative: collected + uncollected)
func (c PnLComponents) NetPnL() float64 {
	return c.LPChange +
		c.HedgeUnrealized +
		c.HedgeRealized +
		c.Funding -
		c.Commissions +
		c.LPFees
}

// MismatchUSD is the LP–hedge mismatch line: the residual once LP fees are
// removed from net PnL. It is LP value change plus the COMPLETE hedge PnL
// (realized + unrealized) and the funding it received, minus commissions —
// i.e. everything except fees. With the complete hedge PnL in place this is the
// true impermanent-loss residual, not the old accounting gap that ignored the
// realized PnL banked out of the open position.
func MismatchUSD(lpChange, hedgeUnrealized, hedgeRealized, funding, commissions float64) float64 {
	return lpChange + CompleteHedgePnL(hedgeRealized, hedgeUnrealized) + funding - commissions
}

// feeLeg tracks one token leg's uncollected fees in TOKEN UNITS for harvest
// detection, plus the running collected (banked) total in token units.
//
// Harvest detection MUST use token units, not USD: uncollected fees in token
// units only ever accrue upward between reads and can only fall on a collect, so
// any decrease is a harvest. The USD value of the same fees can fall just
// because the token price fell, which a USD threshold would mistake for a
// harvest in a token/token pool.
type feeLeg struct {
	Collected   float64 // banked token units harvested since inception (only grows)
	Uncollected float64 // last-seen uncollected token units
	seen        bool    // whether we've recorded a first reading
}

// update feeds the current uncollected token amount and banks a harvest. A
// harvest is any decrease versus the previous read; on one we bank the PREVIOUS
// uncollected amount (the best estimate of what was collected) and then keep
// accruing from the current amount. The cumulative (Collected + Uncollected)
// stays continuous: one part drops by the harvested amount and the other rises
// by the same amount. Fees that accrued in the gap before the collect are
// approximated by banking the last-seen amount; the error is bounded by one
// polling interval.
func (f *feeLeg) update(cur float64) {
	if f.seen && cur < f.Uncollected {
		f.Collected += f.Uncollected
	}
	f.Uncollected = cur
	f.seen = true
}

// cumulative returns the leg's total fees since inception, in token units:
// banked collected plus the current uncollected. Never drops on a harvest.
func (f feeLeg) cumulative() float64 { return f.Collected + f.Uncollected }

// FeeLedger tracks both legs of a position. It is stateful across polls (it must
// remember the previous uncollected amounts to detect a harvest) and lives on
// the tracker.
//
// PERSISTENCE / ACCURACY (documented, not solved here): the collected total is
// held in memory and resets on process restart, like the rest of the strategy
// state, so a restart loses the collected portion until persistence exists. The
// exact source of truth for lifetime collected fees is the on-chain Collect
// events for the position; re-deriving from those, or persisting the running
// total, is the robust later upgrade.
type FeeLedger struct {
	leg0 feeLeg
	leg1 feeLeg
}

// Update records the current uncollected fee amounts of both legs in TOKEN
// UNITS and banks any harvest.
func (l *FeeLedger) Update(uncollected0, uncollected1 float64) {
	l.leg0.update(uncollected0)
	l.leg1.update(uncollected1)
}

// ToCollectUSD is the current uncollected fees in USD — what a harvest would
// realize right now ("fees to collect"). It drops to the post-harvest amount
// when the position is collected.
func (l FeeLedger) ToCollectUSD(price0, price1 float64) float64 {
	return l.leg0.Uncollected*price0 + l.leg1.Uncollected*price1
}

// TotalUSD is the cumulative fees since inception in USD: (collected +
// uncollected) per leg, priced at current prices ("total fees since start").
// It survives harvests and never resets — on a harvest the collected part rises
// by exactly what the uncollected part dropped.
func (l FeeLedger) TotalUSD(price0, price1 float64) float64 {
	return l.leg0.cumulative()*price0 + l.leg1.cumulative()*price1
}
