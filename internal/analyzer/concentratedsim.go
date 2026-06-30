package analyzer

// concentratedsim.go projects the fee APR a $1,000 deposit would earn in a
// ±k·sigma band, as the deposit's SHARE of the real in-range liquidity — the way
// the official LP aggregators present "estimated APR".
//
// The model (share of liquidity):
//
//	delta      = k · sigma · sqrt(T/365)              // log half-width of the band
//	usdPerL    = USD value of one raw unit of L deployed in [priceNow·e^-delta,
//	             priceNow·e^+delta] at the current price
//	Lyou       = deposit / usdPerL                    // your position, in raw L units
//	share      = Lyou / (Lexisting + Lyou)            // Lexisting = real active L at tick
//	annualFees = volume24h · feeTier · 365
//	apr        = annualFees · share / deposit
//
// This is BOUNDED (share ≤ 1, so apr ≤ annualFees/deposit), grounded in the real
// on-chain liquidity, and responds to the band: a tighter band lowers usdPerL, so
// Lyou and the share rise and the APR rises — all capped by the real Lexisting.
//
// When the on-chain liquidity is unavailable (no RPC, unsupported DEX, a probe
// error, or the per-scan probe cap is hit) it falls back to a pool-data-only
// estimate anchored so that at k=1 it equals the pool's full-range fee APR:
//
//	activeUSD = TVL · C(pool ±1sigma band) / C(your ±k band)
//	apr       = annualFees / (activeUSD + deposit)
//
// and tags the result Source="estimated".

import (
	"context"
	"math"
	"math/big"
	"strconv"

	"github.com/filipekdick/lp-tracker-go/internal/onchain"
	"github.com/filipekdick/lp-tracker-go/internal/v3math"
)

// ConcDepositUSD is the hypothetical deposit the concentrated-APR projection is
// computed for.
const ConcDepositUSD = 1000.0

// ConcBandKs are the std-dev band widths the projection is computed for, matching
// the dashboard's ±kσ chips.
var ConcBandKs = []float64{1, 2}

// BandSim is the projected outcome for one ±kσ band.
type BandSim struct {
	APR                float64 `json:"apr"`                // projected fee APR for the deposit
	DailyFeesUSD       float64 `json:"dailyFeesUsd"`       // projected fees/day for the deposit
	ShareOfLiquidity   float64 `json:"share"`              // deposit's share of in-range liquidity
	ActiveLiquidityUSD float64 `json:"activeLiquidityUsd"` // competing in-range liquidity (USD)
	ConcentrationX     float64 `json:"concentrationX"`     // capital efficiency of the band vs full range
	LowerPrice         float64 `json:"lowerPrice"`
	UpperPrice         float64 `json:"upperPrice"`
}

// ConcentratedSim is the concentrated-APR projection for a pool, computed for the
// bands the UI exposes (keys "1" and "2").
type ConcentratedSim struct {
	Source     string             `json:"source"` // "onchain" | "estimated"
	DepositUSD float64            `json:"depositUsd"`
	Bands      map[string]BandSim `json:"bands"`
}

// ConcentratedInput carries the per-pool context the projection needs.
type ConcentratedInput struct {
	ChainSlug    string
	PoolAddress  string
	DEX          string
	FeeTier      float64
	TVLUSD       float64
	Volume24hUSD float64
	RealizedVol  float64
	BaseUSD      float64 // datasource base-token USD price
	QuoteUSD     float64 // datasource quote-token USD price
	HorizonDays  float64
}

// ConcentratedProjection computes the concentrated-APR projection for a pool. It
// uses the on-chain prober as the primary path (when allowProbe is true, the DEX
// is a supported v3 fork and the probe succeeds), otherwise the bounded pool-data
// fallback. Returns nil when there is no usable volatility signal or fee flow, so
// it never errors out the scan.
func ConcentratedProjection(ctx context.Context, prober onchain.Prober, in ConcentratedInput, allowProbe bool) *ConcentratedSim {
	if in.RealizedVol <= 0 || in.Volume24hUSD <= 0 || in.FeeTier <= 0 {
		return nil
	}
	horizon := in.HorizonDays
	if horizon <= 0 {
		horizon = DefaultHorizonDays
	}

	if allowProbe && prober != nil && in.ChainSlug != "" && in.PoolAddress != "" && onchain.SupportedDEX(in.DEX) {
		if st, ok, err := prober.PoolStateAt(ctx, in.ChainSlug, in.PoolAddress); err == nil && ok {
			if sim := onchainSim(st, in, horizon); sim != nil {
				return sim
			}
		}
	}
	return estimatedSim(in, horizon)
}

// onchainSim computes the share-of-liquidity projection from real pool state.
// Returns nil if the state can't be valued (so the caller falls back).
func onchainSim(st onchain.PoolState, in ConcentratedInput, horizon float64) *ConcentratedSim {
	if st.SqrtPriceX96 == nil || st.SqrtPriceX96.Sign() <= 0 {
		return nil
	}
	priceNow := v3math.PriceAtTick(st.TickNow, st.Dec0, st.Dec1)
	if priceNow <= 0 {
		return nil
	}
	price0USD, price1USD, ok := alignUSD(priceNow, in.BaseUSD, in.QuoteUSD)
	if !ok {
		return nil
	}
	annualFees := in.Volume24hUSD * in.FeeTier * 365.0
	lExisting := bigToFloat(st.Liquidity)
	if lExisting < 0 {
		lExisting = 0
	}

	bands := make(map[string]BandSim, len(ConcBandKs))
	for _, k := range ConcBandKs {
		delta := LogHalfWidth(in.RealizedVol, horizon, k)
		if delta <= 0 {
			return nil
		}
		lower := priceNow * math.Exp(-delta)
		upper := priceNow * math.Exp(delta)
		tickLower := v3math.TickAtPrice(lower, st.Dec0, st.Dec1)
		tickUpper := v3math.TickAtPrice(upper, st.Dec0, st.Dec1)
		if tickLower >= tickUpper {
			return nil
		}
		// USD value of one raw unit of L deployed across the band at the current
		// price. Use a large Lunit (1e18) for precision, then divide back out.
		lUnit := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		amt0, amt1 := v3math.GetAmountsForLiquidity(st.SqrtPriceX96, tickLower, tickUpper, lUnit)
		human0 := bigToFloat(amt0) / math.Pow(10, float64(st.Dec0))
		human1 := bigToFloat(amt1) / math.Pow(10, float64(st.Dec1))
		usdPerL := (human0*price0USD + human1*price1USD) / 1e18
		if usdPerL <= 0 {
			return nil
		}
		lYou := ConcDepositUSD / usdPerL
		share := lYou / (lExisting + lYou)
		apr := annualFees * share / ConcDepositUSD
		bands[strconv.Itoa(int(k))] = BandSim{
			APR:                apr,
			DailyFeesUSD:       apr * ConcDepositUSD / 365.0,
			ShareOfLiquidity:   share,
			ActiveLiquidityUSD: lExisting * usdPerL,
			ConcentrationX:     ConcentrationFactor(delta),
			LowerPrice:         lower,
			UpperPrice:         upper,
		}
	}
	return &ConcentratedSim{Source: "onchain", DepositUSD: ConcDepositUSD, Bands: bands}
}

// estimatedSim is the bounded pool-data-only fallback (see file header). At k=1
// the APR equals the pool's full-range fee APR.
func estimatedSim(in ConcentratedInput, horizon float64) *ConcentratedSim {
	if in.TVLUSD <= 0 {
		return nil
	}
	annualFees := in.Volume24hUSD * in.FeeTier * 365.0
	priceNow := 0.0
	if in.BaseUSD > 0 && in.QuoteUSD > 0 {
		priceNow = in.BaseUSD / in.QuoteUSD
	}
	deltaPool := LogHalfWidth(in.RealizedVol, horizon, 1)
	if deltaPool <= 0 {
		return nil
	}
	cPool := ConcentrationFactor(deltaPool)

	bands := make(map[string]BandSim, len(ConcBandKs))
	for _, k := range ConcBandKs {
		deltaYou := LogHalfWidth(in.RealizedVol, horizon, k)
		if deltaYou <= 0 {
			return nil
		}
		cYou := ConcentrationFactor(deltaYou)
		activeUSD := in.TVLUSD * cPool / cYou
		apr := annualFees / (activeUSD + ConcDepositUSD)
		lower, upper := 0.0, 0.0
		if priceNow > 0 {
			lower = priceNow * math.Exp(-deltaYou)
			upper = priceNow * math.Exp(deltaYou)
		}
		bands[strconv.Itoa(int(k))] = BandSim{
			APR:                apr,
			DailyFeesUSD:       apr * ConcDepositUSD / 365.0,
			ShareOfLiquidity:   ConcDepositUSD / (activeUSD + ConcDepositUSD),
			ActiveLiquidityUSD: activeUSD,
			ConcentrationX:     cYou,
			LowerPrice:         lower,
			UpperPrice:         upper,
		}
	}
	return &ConcentratedSim{Source: "estimated", DepositUSD: ConcDepositUSD, Bands: bands}
}

// alignUSD assigns the datasource base/quote USD prices to token0/token1 so they
// match the on-chain ordering, using the current ratio price (token1 per token0,
// which equals USD(token0)/USD(token1)). It picks the assignment whose implied
// ratio is closest to priceNow in log space. ok is false when either USD price is
// non-positive (the pool can't be valued, so the caller falls back).
func alignUSD(priceNow, baseUSD, quoteUSD float64) (price0USD, price1USD float64, ok bool) {
	if priceNow <= 0 || baseUSD <= 0 || quoteUSD <= 0 {
		return 0, 0, false
	}
	rBaseIs0 := baseUSD / quoteUSD  // if token0 == base, priceNow ≈ this
	rQuoteIs0 := quoteUSD / baseUSD // if token0 == quote, priceNow ≈ this
	if math.Abs(math.Log(rBaseIs0/priceNow)) <= math.Abs(math.Log(rQuoteIs0/priceNow)) {
		return baseUSD, quoteUSD, true
	}
	return quoteUSD, baseUSD, true
}

// bigToFloat converts a big.Int to float64 (0 for nil). Precision loss is
// irrelevant here: these feed ratios and USD products, not exact accounting.
func bigToFloat(x *big.Int) float64 {
	if x == nil {
		return 0
	}
	f, _ := new(big.Float).SetInt(x).Float64()
	return f
}
