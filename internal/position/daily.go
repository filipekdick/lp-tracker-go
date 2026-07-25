package position

import (
	"sort"
	"time"
)

// DailyReturn is one UTC day's strategy result, split into independently
// auditable income and cost components. TradingFeesPaidUSD is a positive cost;
// every other field is signed income/PnL.
type DailyReturn struct {
	Date                  string  `json:"date"`
	StartValueUSD         float64 `json:"startValueUsd"`
	LPValueChangeUSD      float64 `json:"lpValueChangeUsd"`
	LPFeesUSD             float64 `json:"lpFeesUsd"`
	GaugeRewardsUSD       float64 `json:"gaugeRewardsUsd"`
	GaugeRewardsAvailable bool    `json:"gaugeRewardsAvailable"`
	HedgePnLUSD           float64 `json:"hedgePnlUsd"`
	FundingUSD            float64 `json:"fundingUsd"`
	TradingFeesPaidUSD    float64 `json:"tradingFeesPaidUsd"`
	NetReturnUSD          float64 `json:"netReturnUsd"`
	ReturnPct             float64 `json:"returnPct"`
}

// BuildDailyReturns rolls cumulative strategy snapshots into UTC calendar days.
// Each component is the end-minus-start delta for that day, preventing funding,
// fees or realized hedge PnL from being counted more than once.
func BuildDailyReturns(initial *Snapshot, history []Snapshot) []DailyReturn {
	if initial == nil || len(history) == 0 {
		return nil
	}
	points := append([]Snapshot(nil), history...)
	sort.SliceStable(points, func(i, j int) bool { return points[i].Timestamp.Before(points[j].Timestamp) })

	start := *initial
	end := start
	day := utcDay(points[0].Timestamp)
	var out []DailyReturn
	for _, point := range points {
		pointDay := utcDay(point.Timestamp)
		if pointDay != day {
			out = append(out, dailyDelta(day, start, end))
			start = end
			day = pointDay
		}
		end = point
	}
	out = append(out, dailyDelta(day, start, end))
	return out
}

func utcDay(t time.Time) string { return t.UTC().Format("2006-01-02") }

func dailyDelta(day string, start, end Snapshot) DailyReturn {
	d := DailyReturn{
		Date:                  day,
		StartValueUSD:         start.ValueUSD,
		LPValueChangeUSD:      end.ValueUSD - start.ValueUSD,
		LPFeesUSD:             end.FeesUSD - start.FeesUSD,
		GaugeRewardsUSD:       end.GaugeRewardsUSD - start.GaugeRewardsUSD,
		GaugeRewardsAvailable: start.GaugeRewardsAvailable || end.GaugeRewardsAvailable,
		HedgePnLUSD: (end.HedgePnL + end.HedgeRealizedPnL) -
			(start.HedgePnL + start.HedgeRealizedPnL),
		FundingUSD:         end.HedgeFundingUSD - start.HedgeFundingUSD,
		TradingFeesPaidUSD: end.HedgeCommissionsUSD - start.HedgeCommissionsUSD,
	}
	d.NetReturnUSD = d.LPValueChangeUSD + d.LPFeesUSD + d.GaugeRewardsUSD +
		d.HedgePnLUSD + d.FundingUSD - d.TradingFeesPaidUSD
	if d.StartValueUSD > 0 {
		d.ReturnPct = d.NetReturnUSD / d.StartValueUSD
	}
	return d
}
