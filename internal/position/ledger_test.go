package position

import (
	"context"
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestCompleteHedgePnLDisjoint constructs a case with both a closed leg
// (realized) and an open position (unrealized) and checks the complete hedge
// PnL is exactly their sum — they are disjoint and add without double-counting.
func TestCompleteHedgePnLDisjoint(t *testing.T) {
	realized := 120.0   // banked when an earlier leg was rebalanced out
	unrealized := -45.0 // the currently open short is underwater
	got := CompleteHedgePnL(realized, unrealized)
	if !approx(got, 75.0) {
		t.Fatalf("complete hedge PnL = %v, want 75.0", got)
	}
	// Disjointness: changing one does not change the other's contribution.
	if !approx(CompleteHedgePnL(realized, 0)+CompleteHedgePnL(0, unrealized), got) {
		t.Fatal("realized and unrealized are not adding independently")
	}
}

// TestNetPnLIdentity checks the full net PnL identity on a constructed case with
// known LP change, unrealized, realized, funding, commissions and fees, with
// every sign exercised (a loss-making unrealized, a paid commission cost).
func TestNetPnLIdentity(t *testing.T) {
	c := PnLComponents{
		LPChange:        -30.0, // LP lost value (impermanent loss)
		HedgeUnrealized: 18.0,  // open short up
		HedgeRealized:   25.0,  // banked realized gains
		Funding:         4.0,   // short received funding
		Commissions:     3.5,   // paid (subtracted)
		LPFees:          12.0,  // fees earned
	}
	// -30 + 18 + 25 + 4 - 3.5 + 12 = 25.5
	if got := c.NetPnL(); !approx(got, 25.5) {
		t.Fatalf("net PnL = %v, want 25.5", got)
	}
}

// TestMismatchLine checks the LP–hedge mismatch equals LP value change plus the
// complete hedge PnL (realized + unrealized) plus funding, minus commissions —
// i.e. net PnL with the fee term removed.
func TestMismatchLine(t *testing.T) {
	lpChange, unreal, realized, funding, comm, fees := -30.0, 18.0, 25.0, 4.0, 3.5, 12.0
	mismatch := MismatchUSD(lpChange, unreal, realized, funding, comm)
	if !approx(mismatch, 13.5) { // -30 + (25+18) + 4 - 3.5
		t.Fatalf("mismatch = %v, want 13.5", mismatch)
	}
	// Equivalence: mismatch == netPnL − fees.
	net := PnLComponents{lpChange, unreal, realized, funding, comm, fees}.NetPnL()
	if !approx(mismatch, net-fees) {
		t.Fatalf("mismatch %v != netPnL-fees %v", mismatch, net-fees)
	}
}

// TestFeeLedgerSurvivesHarvest constructs a sequence where a leg's uncollected
// token amount accrues, then drops (a harvest). The cumulative total must stay
// continuous (never fall) while "fees to collect" drops to the post-harvest
// amount.
func TestFeeLedgerSurvivesHarvest(t *testing.T) {
	const p0, p1 = 2.0, 1.0 // constant prices to isolate the harvest behaviour
	var l FeeLedger

	l.Update(1.0, 10.0) // accrue
	l.Update(3.0, 25.0) // accrue more
	beforeTotal := l.TotalUSD(p0, p1)
	beforeToCollect := l.ToCollectUSD(p0, p1)
	if !approx(beforeToCollect, 3*p0+25*p1) {
		t.Fatalf("pre-harvest to-collect = %v", beforeToCollect)
	}

	// Harvest leg0: uncollected token amount falls 3.0 -> 0.2. Leg1 keeps accruing.
	l.Update(0.2, 26.0)
	afterTotal := l.TotalUSD(p0, p1)
	afterToCollect := l.ToCollectUSD(p0, p1)

	// Cumulative must not fall across the harvest (it should rise slightly: leg1
	// accrued +1 token, leg0's banked 3.0 replaces its dropped uncollected).
	if afterTotal < beforeTotal-1e-9 {
		t.Fatalf("cumulative fell across harvest: %v -> %v", beforeTotal, afterTotal)
	}
	// "Fees to collect" must drop to the post-harvest uncollected amount.
	wantToCollect := 0.2*p0 + 26.0*p1
	if !approx(afterToCollect, wantToCollect) {
		t.Fatalf("post-harvest to-collect = %v, want %v", afterToCollect, wantToCollect)
	}
	// The two fields must now differ (collected was banked).
	if approx(afterTotal, afterToCollect) {
		t.Fatal("after a harvest, total and to-collect should differ")
	}
	// Banked collected = the previous uncollected (3.0 tokens of leg0).
	if !approx(afterTotal-afterToCollect, 3.0*p0) {
		t.Fatalf("banked collected USD = %v, want %v", afterTotal-afterToCollect, 3.0*p0)
	}
}

// TestHarvestDetectionTokenUnitsNotUSD checks a price-only drop in fee USD value
// (token amounts unchanged) is NOT counted as a harvest and does not change the
// collected total.
func TestHarvestDetectionTokenUnitsNotUSD(t *testing.T) {
	var l FeeLedger
	l.Update(5.0, 0.0) // 5 tokens of leg0
	highPrice := l.TotalUSD(10.0, 1.0)
	l.Update(5.0, 0.0) // same token amounts, price about to fall
	lowPrice := l.TotalUSD(4.0, 1.0)

	// USD value fell purely because the price fell.
	if !(lowPrice < highPrice) {
		t.Fatal("expected USD fee value to fall with price")
	}
	// But no harvest was banked: to-collect == total (nothing collected).
	if !approx(l.TotalUSD(4.0, 1.0), l.ToCollectUSD(4.0, 1.0)) {
		t.Fatal("price-only drop must not bank a harvest")
	}
	if !approx(l.leg0.Collected, 0) || !approx(l.leg1.Collected, 0) {
		t.Fatalf("collected changed on a price-only drop: leg0=%v leg1=%v", l.leg0.Collected, l.leg1.Collected)
	}
}

// TestFeeCardTwoDistinctFields checks the fee card's two numbers — "fees to
// collect" (uncollected) and "total fees since start" (cumulative) — are
// exposed and differ after a harvest.
func TestFeeCardTwoDistinctFields(t *testing.T) {
	tp, err := NewDemoTracker(1).Track(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tp.FeesToCollectUSD <= 0 || tp.FeesTotalUSD <= 0 {
		t.Fatalf("expected both fee fields populated, got toCollect=%v total=%v",
			tp.FeesToCollectUSD, tp.FeesTotalUSD)
	}
	if tp.FeesTotalUSD <= tp.FeesToCollectUSD {
		t.Fatalf("total fees %v should exceed fees-to-collect %v after a harvest",
			tp.FeesTotalUSD, tp.FeesToCollectUSD)
	}
}

// TestDemoRendersIncomeFields checks demo mode populates the new realized,
// funding and commission fields (and marks the ledger complete, not partial).
func TestDemoRendersIncomeFields(t *testing.T) {
	tp, err := NewDemoTracker(1).Track(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tp.HedgeRealizedPnL == 0 {
		t.Error("expected non-zero demo realized PnL")
	}
	if tp.HedgeFundingUSD == 0 {
		t.Error("expected non-zero demo funding")
	}
	if tp.HedgeCommissionsUSD <= 0 {
		t.Error("expected positive demo commissions (a paid cost)")
	}
	if tp.HedgeIncomePartial {
		t.Error("demo ledger should not be partial")
	}
	// The demo history's last NetPnL should match the identity recomputed from
	// its own snapshot components against the baseline.
	if len(tp.History) == 0 || tp.InitialState == nil {
		t.Fatal("expected demo history and baseline")
	}
	last := tp.History[len(tp.History)-1]
	init := tp.InitialState
	want := PnLComponents{
		LPChange:        last.ValueUSD - init.ValueUSD,
		HedgeUnrealized: last.HedgePnL - init.HedgePnL,
		HedgeRealized:   last.HedgeRealizedPnL,
		Funding:         last.HedgeFundingUSD,
		Commissions:     last.HedgeCommissionsUSD,
		LPFees:          last.FeesUSD - init.FeesUSD,
	}.NetPnL()
	if !approx(last.NetPnL, want) {
		t.Fatalf("demo net PnL %v != identity %v", last.NetPnL, want)
	}
}
