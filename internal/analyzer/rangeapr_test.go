package analyzer

import (
	"math"
	"testing"
)

func TestRangeFeeAPRShareModel(t *testing.T) {
	// volume*feeTier*365 / (active + deposit).
	// 1,000,000 * 0.0005 * 365 = 182,500 annual fees.
	// active 1,000,000, deposit 0 -> 0.1825 marginal APR.
	got := RangeFeeAPR(1_000_000, 0.0005, 0, 1_000_000)
	if math.Abs(got-0.1825) > 1e-9 {
		t.Fatalf("marginal APR: got %v want 0.1825", got)
	}
	// A deposit equal to the active liquidity halves the APR (you own half the range).
	got = RangeFeeAPR(1_000_000, 0.0005, 1_000_000, 1_000_000)
	if math.Abs(got-0.09125) > 1e-9 {
		t.Fatalf("self-diluted APR: got %v want 0.09125", got)
	}
	// No fee flow -> zero.
	if RangeFeeAPR(0, 0.0005, 0, 1_000_000) != 0 {
		t.Fatal("zero volume must give zero APR")
	}
}

func TestActiveLiquidityShrinksWithRange(t *testing.T) {
	// A tighter range competes against less of the TVL than a wider one.
	tight := ActiveLiquidityInRangeUSD(1_000_000, 95, 105) // ±5%
	wide := ActiveLiquidityInRangeUSD(1_000_000, 50, 200)  // wide
	if !(tight < wide) {
		t.Fatalf("tighter range should have less active liquidity: tight=%v wide=%v", tight, wide)
	}
	if tight <= 0 || tight >= 1_000_000 {
		t.Fatalf("active liquidity should be a positive fraction of TVL, got %v", tight)
	}
}

func TestMarginalAPRIsConcentrationBoostedFullRange(t *testing.T) {
	// With deposit -> 0 the projected APR must equal fullRangeAPR * C(delta).
	const vol, fee, tvl, price = 2_000_000.0, 0.0003, 5_000_000.0, 100.0
	lower, upper := 90.0, 110.0
	sim := SimulateRangeAPRPrices(vol, fee, tvl, price, lower, upper, 0)

	fullRangeAPR := FeeAPR(vol, fee, tvl)
	want := fullRangeAPR * sim.ConcentrationX
	if math.Abs(sim.FeeAPR-want) > 1e-9 {
		t.Fatalf("marginal range APR: got %v want fullRange*C = %v", sim.FeeAPR, want)
	}
	if sim.ConcentrationX <= 1 {
		t.Fatalf("a concentrated range should have C>1, got %v", sim.ConcentrationX)
	}
	// The concentrated projection must beat the full-range APR.
	if !(sim.FeeAPR > fullRangeAPR) {
		t.Fatalf("concentrated APR %v should exceed full-range %v", sim.FeeAPR, fullRangeAPR)
	}
}

func TestSimulateVolRangeAdaptsToVolatility(t *testing.T) {
	// Lower volatility -> tighter vol-sized range -> more concentration -> higher APR.
	const vol24h, fee, tvl, price, horizon, z, deposit = 1_000_000.0, 0.0005, 2_000_000.0, 3000.0, 1.0, 1.0, 10_000.0
	low := SimulateVolRange(vol24h, fee, tvl, price, 0.30, horizon, z, deposit)
	high := SimulateVolRange(vol24h, fee, tvl, price, 0.90, horizon, z, deposit)
	if !(low.FeeAPR > high.FeeAPR) {
		t.Fatalf("lower vol should give higher concentrated APR: low=%v high=%v", low.FeeAPR, high.FeeAPR)
	}
	if low.LowerPrice <= 0 || low.UpperPrice <= low.LowerPrice {
		t.Fatalf("vol range prices invalid: %+v", low)
	}
	if low.DailyFeesUSD <= 0 {
		t.Fatalf("expected positive daily fees, got %v", low.DailyFeesUSD)
	}
}

func TestAnalyzeAttachesRangeSim(t *testing.T) {
	// A pool with fee flow and a volatility signal should carry a range sim.
	closes := make([]float64, 0, 200)
	p := 100.0
	for i := 0; i < 200; i++ {
		// gentle deterministic oscillation so realized vol > 0
		p *= 1 + 0.01*math.Sin(float64(i))
		closes = append(closes, p)
	}
	res := Analyze(Input{
		FeeTier:        0.0005,
		TVLUSD:         2_000_000,
		Volume24hUSD:   1_500_000,
		Closes:         closes,
		PeriodsPerYear: 24 * 365,
	})
	if res.RangeSim == nil {
		t.Fatal("expected a range simulation to be attached")
	}
	if res.RangeSim.FeeAPR <= 0 || res.RangeSim.ConcentrationX <= 1 {
		t.Fatalf("range sim looks wrong: %+v", res.RangeSim)
	}
}
