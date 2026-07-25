package position

import (
	"testing"
	"time"
)

func TestBuildDailyReturnsSplitsCumulativeComponents(t *testing.T) {
	at := func(day int, hour int) time.Time {
		return time.Date(2026, time.July, day, hour, 0, 0, 0, time.UTC)
	}
	initial := &Snapshot{Timestamp: at(1, 0), ValueUSD: 1000, HedgePnL: 10}
	history := []Snapshot{
		{Timestamp: at(1, 23), ValueUSD: 1020, HedgePnL: 8, HedgeRealizedPnL: 5, HedgeFundingUSD: 2, HedgeCommissionsUSD: 1, FeesUSD: 12, GaugeRewardsUSD: 3, GaugeRewardsAvailable: true},
		{Timestamp: at(2, 23), ValueUSD: 1010, HedgePnL: 15, HedgeRealizedPnL: 7, HedgeFundingUSD: 1, HedgeCommissionsUSD: 2.5, FeesUSD: 20, GaugeRewardsUSD: 5, GaugeRewardsAvailable: true},
	}

	got := BuildDailyReturns(initial, history)
	if len(got) != 2 {
		t.Fatalf("got %d days, want 2", len(got))
	}
	// Day 1: LP +20, fees +12, gauge +3, hedge +(8+5-10)=+3,
	// funding +2, trading fees -1 => net +39.
	if !approx(got[0].NetReturnUSD, 39) || !approx(got[0].ReturnPct, 0.039) {
		t.Fatalf("day 1 = %+v, want net 39 and return 3.9%%", got[0])
	}
	// Day 2 starts at day 1's close and therefore only contains day 2 deltas.
	// LP -10, fees +8, gauge +2, hedge +(15+7-8-5)=+9,
	// funding -1, trading fees -1.5 => net +6.5.
	if !approx(got[1].NetReturnUSD, 6.5) {
		t.Fatalf("day 2 = %+v, want net 6.5", got[1])
	}
	if !got[1].GaugeRewardsAvailable {
		t.Fatal("expected gauge rewards to be marked available")
	}
}
