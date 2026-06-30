package v3math

import (
	"math"
	"testing"
)

// TestPriceAtTick checks the tick→price conversion against hand-computed values.
func TestPriceAtTick(t *testing.T) {
	// tick 0 with equal decimals => price 1.0.
	if p := PriceAtTick(0, 18, 18); math.Abs(p-1) > 1e-12 {
		t.Fatalf("PriceAtTick(0) = %v, want 1", p)
	}

	// WETH(18)/USDC(6) at tick -200000:
	// price = 1.0001^-200000 * 10^(18-6)
	//       = exp(-200000*ln(1.0001)) * 1e12
	// ln(1.0001) = 9.99950003e-5 -> exponent -19.99900007 -> exp = 2.0601e-9
	// * 1e12 = 2060.1 (USDC per WETH).
	want := math.Pow(1.0001, -200000) * math.Pow(10, 12)
	got := PriceAtTick(-200000, 18, 6)
	if math.Abs(got-want)/want > 1e-9 {
		t.Fatalf("PriceAtTick(-200000,18,6) = %v, want %v", got, want)
	}
	if got < 1500 || got > 2500 {
		t.Fatalf("WETH/USDC price at tick -200000 = %v, expected ~2060", got)
	}
}

// TestTickAtPriceRoundTrips checks TickAtPrice inverts PriceAtTick to within the
// rounding of one tick, across decimal layouts and signs.
func TestTickAtPriceRoundTrips(t *testing.T) {
	cases := []struct {
		tick       int64
		dec0, dec1 uint8
	}{
		{0, 18, 18},
		{-200000, 18, 6},  // WETH/USDC
		{200000, 6, 18},   // reversed decimals
		{-2000, 18, 18},   // wstETH/WETH-ish
		{123456, 8, 18},   // cbBTC/WETH-ish
		{-887000, 18, 18}, // near min
	}
	for _, c := range cases {
		price := PriceAtTick(c.tick, c.dec0, c.dec1)
		got := TickAtPrice(price, c.dec0, c.dec1)
		if d := got - c.tick; d < -1 || d > 1 {
			t.Fatalf("TickAtPrice(PriceAtTick(%d)) = %d, want within ±1", c.tick, got)
		}
	}
	// Non-positive price is handled, not panicking.
	if TickAtPrice(0, 18, 18) != 0 {
		t.Fatal("TickAtPrice(0) should be 0")
	}
}

// TestRangePricesWithin checks the within-range reads 0% at the lower tick and
// 100% at the upper tick, and matches hand-computed endpoint prices. No network.
func TestRangePricesWithin(t *testing.T) {
	const (
		lower = int64(-201_400)
		upper = int64(-199_800)
	)
	// At the lower bound.
	lo := RangePrices(lower, upper, lower, 18, 6)
	if math.Abs(lo.WithinPct-0) > 1e-9 {
		t.Fatalf("within at lower bound = %v, want 0", lo.WithinPct)
	}
	if math.Abs(lo.CurrentPrice-PriceAtTick(lower, 18, 6)) > 1e-12 {
		t.Fatalf("current price at lower bound mismatch")
	}
	// At the upper bound.
	hi := RangePrices(lower, upper, upper, 18, 6)
	if math.Abs(hi.WithinPct-100) > 1e-9 {
		t.Fatalf("within at upper bound = %v, want 100", hi.WithinPct)
	}
	// Midpoint by ticks => 50%.
	mid := (lower + upper) / 2
	m := RangePrices(lower, upper, mid, 18, 6)
	if math.Abs(m.WithinPct-50) > 0.1 {
		t.Fatalf("within at midpoint tick = %v, want ~50", m.WithinPct)
	}
	// Lower price < upper price (price increases with tick).
	if !(lo.LowerPrice < lo.UpperPrice) {
		t.Fatalf("lower price %v should be < upper price %v", lo.LowerPrice, lo.UpperPrice)
	}
}
