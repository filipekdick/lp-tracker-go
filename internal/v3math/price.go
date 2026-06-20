package v3math

import "math"

// PriceAtTick returns the human-readable price of token0 denominated in token1
// (i.e. token1 per token0) at the given tick, accounting for token decimals.
//
// In Uniswap/Aerodrome v3 the raw price (token1/token0 in smallest units) at a
// tick is 1.0001^tick. Converting to human units multiplies by 10^(dec0-dec1):
//
//	price_human = 1.0001^tick * 10^(dec0 - dec1)
//
// This is display-only (float64) and needs no network or chain access — the tick
// fully determines the price.
func PriceAtTick(tick int64, dec0, dec1 uint8) float64 {
	return math.Pow(1.0001, float64(tick)) * math.Pow(10, float64(int(dec0)-int(dec1)))
}

// RangeInfo describes a position's range in notional prices plus where the
// current price sits within it.
type RangeInfo struct {
	LowerPrice   float64 // price at tickLower (token1 per token0)
	UpperPrice   float64 // price at tickUpper
	CurrentPrice float64 // price at tickNow
	// WithinPct is the current price's position within the range in TICK terms:
	// 0% at the lower bound, 100% at the upper bound. Tick space is linear in
	// log-price, which is the natural measure for "how far through the range" and
	// makes the endpoints land exactly on 0/100. It is NOT clamped here.
	WithinPct float64
}

// RangePrices computes a position's range prices and within-range position from
// its ticks and the two tokens' decimals. No network or chain access.
func RangePrices(tickLower, tickUpper, tickNow int64, dec0, dec1 uint8) RangeInfo {
	info := RangeInfo{
		LowerPrice:   PriceAtTick(tickLower, dec0, dec1),
		UpperPrice:   PriceAtTick(tickUpper, dec0, dec1),
		CurrentPrice: PriceAtTick(tickNow, dec0, dec1),
	}
	if tickUpper != tickLower {
		info.WithinPct = float64(tickNow-tickLower) / float64(tickUpper-tickLower) * 100.0
	}
	return info
}
