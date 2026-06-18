package position

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
)

// DemoTracker synthesises a realistic Aerodrome WETH/USDC position on Base,
// complete with a Binance perp short hedge, so the dashboard runs offline.
type DemoTracker struct {
	tokenID int64
	iv      datasource.ImpliedVolSource
}

// NewDemoTracker builds a demo tracker for the given token ID.
func NewDemoTracker(tokenID int64) *DemoTracker {
	return &DemoTracker{tokenID: tokenID, iv: datasource.DemoImpliedVol{}}
}

func (t *DemoTracker) Name() string { return "demo" }

// Track returns a synthetic but internally-consistent tracked position. The
// price history is regenerated each call (seeded by the clock) so the hedge
// drift and PnL move between refreshes, mimicking a live position.
func (t *DemoTracker) Track(ctx context.Context) (TrackedPosition, error) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	const (
		entryPrice = 3380.0
		ethVol     = 0.55
		feeTier    = 0.0005 // 5 bps Aerodrome slipstream pool
	)

	// 7 days of hourly WETH prices ending near the live mark.
	closes := geometricBrownianDemo(rng, entryPrice, ethVol, 168)
	mark := closes[len(closes)-1]

	// Position sits in a ±8% range around the entry; currently in range.
	wethAmount := 4.0 + rng.Float64()*0.6
	usdcAmount := wethAmount * mark * (0.85 + rng.Float64()*0.2)
	valueUSD := wethAmount*mark + usdcAmount

	tvl := 18_000_000 + rng.Float64()*6_000_000
	volume := tvl * (0.9 + rng.Float64()*0.8) // healthy turnover -> attractive fees

	rp := datasource.RawPool{
		Chain:          "Base",
		ChainSlug:      "base",
		ChainKind:      datasource.L2,
		Address:        "0xb2cc224c1c9feE385f8ad6a55b4d94E92359DC59",
		DEX:            "aerodrome_slipstream",
		Protocol:       "Aerodrome",
		Name:           "WETH / USDC",
		BaseSymbol:     "WETH",
		QuoteSymbol:    "USDC",
		FeeTier:        feeTier,
		TVLUSD:         tvl,
		Volume24hUSD:   volume,
		PriceUSD:       mark,
		QuotePriceUSD:  1,
		Closes:         closes,
		PeriodsPerYear: 24 * 365,
	}

	res := runAnalysis(ctx, rp, t.iv)

	// Hedge: short WETH exposure on Binance, deliberately a touch out of sync.
	current := wethAmount - (0.05 + rng.Float64()*0.25)
	hedge := buildHedge("WETH", "ETHUSDT", wethAmount, current, entryPrice, mark, true, true,
		"Binance Futures (testnet) · demo")

	tp := TrackedPosition{
		TokenID:      t.tokenID,
		Chain:        rp.Chain,
		ChainSlug:    rp.ChainSlug,
		ChainKind:    rp.ChainKind,
		Protocol:     rp.Protocol,
		DEX:          rp.DEX,
		PoolAddress:  rp.Address,
		PoolName:     rp.Name,
		Symbol0:      "WETH",
		Symbol1:      "USDC",
		Amount0:      wethAmount,
		Amount1:      usdcAmount,
		TickLower:    -201_400,
		TickUpper:    -199_800,
		TickNow:      -200_600,
		InRange:      true,
		ValueUSD:     valueUSD,
		FeeTier:      rp.FeeTier,
		TVLUSD:       rp.TVLUSD,
		Volume24hUSD: rp.Volume24hUSD,
		Analysis:     res,
		Hedges:       []Hedge{hedge},
		Hedge:        hedge,
		Source:       "demo",
		UpdatedAt:    time.Now(),
	}

	// Synthetic open futures positions so the dashboard's "Open futures shorts"
	// table has rows to show. The ETH leg mirrors the hedge above; the others
	// give the table some variety.
	tp.OpenShorts = demoOpenShorts(rng, current, entryPrice, mark)

	// Synthetic strategy history so the Net-PnL/fees graph has a timeline to
	// draw before any live scans land. Live tracking builds this incrementally;
	// here we backfill a believable few hours in one shot.
	tp.InitialState, tp.History = demoHistory(rng, closes, wethAmount, usdcAmount, current, entryPrice)

	return tp, nil
}

// demoOpenShorts builds a handful of synthetic Binance positions for the
// open-shorts table. Short PnL follows size*(mark-entry) (size is negative).
func demoOpenShorts(rng *rand.Rand, ethShort, ethEntry, ethMark float64) []binance.Position {
	shorts := []binance.Position{
		{Symbol: "ETHUSDT", Size: -ethShort, EntryPrice: ethEntry, MarkPrice: ethMark},
		{Symbol: "BTCUSDT", Size: -(0.04 + rng.Float64()*0.08), EntryPrice: 64000, MarkPrice: 64000 * (0.97 + rng.Float64()*0.06)},
		{Symbol: "SOLUSDT", Size: -(8 + rng.Float64()*20), EntryPrice: 150, MarkPrice: 150 * (0.95 + rng.Float64()*0.1)},
	}
	for i := range shorts {
		s := &shorts[i]
		s.UnrealizedPnL = s.Size * (s.MarkPrice - s.EntryPrice)
		s.LiquidationPrice = s.EntryPrice * 1.8 // rough short liquidation level
	}
	return shorts
}

// demoHistory backfills a few hours of strategy snapshots ending now, so the
// graph paints a line on first load. NetPnL = LP value change + hedge PnL +
// accrued fees, matching the live tracker's definition; the initial state is
// the baseline (NetPnL zero).
func demoHistory(rng *rand.Rand, closes []float64, wethAmount, usdcAmount, short, entry float64) (*Snapshot, []Snapshot) {
	const n = 24
	const step = 8 * time.Minute
	if len(closes) < n {
		return nil, nil
	}
	start := time.Now().Add(-(n - 1) * step)
	tail := closes[len(closes)-n:]
	value0 := wethAmount*tail[0] + usdcAmount
	hedge0 := short * (entry - tail[0]) // baseline hedge PnL at inception

	feesTotal := 6 + rng.Float64()*40 // USD of fees accrued over the window
	hist := make([]Snapshot, 0, n)
	for i := 0; i < n; i++ {
		progress := float64(i) / float64(n-1)
		price := tail[i]
		value := wethAmount*price + usdcAmount
		hedgePnL := short * (entry - price) // short gains as price falls
		fees := feesTotal * progress
		// Net PnL is the change since inception (see live.go), so the curve
		// starts at zero rather than at the hedge's standing PnL.
		hist = append(hist, Snapshot{
			Timestamp: start.Add(time.Duration(i) * step),
			Price0:    price,
			Price1:    1,
			ValueUSD:  value,
			HedgePnL:  hedgePnL,
			FeesUSD:   fees,
			NetPnL:    (value - value0) + (hedgePnL - hedge0) + fees,
		})
	}
	initial := hist[0]
	initial.NetPnL = 0
	return &initial, hist
}

// buildHedge assembles a Hedge from a target/current short and prices.
func buildHedge(asset, perp string, target, current, entry, mark float64, dryRun, available bool, note string) Hedge {
	if current < 0 {
		current = 0
	}
	drift := target - current
	// Short PnL: gains when price falls below entry.
	pnl := (entry - mark) * current
	h := Hedge{
		Venue:          "Binance Futures",
		Symbol:         perp,
		ExposureSymbol: asset,
		LPExposure:     target,
		TargetShort:    target,
		CurrentShort:   current,
		Drift:          drift,
		EntryPrice:     entry,
		MarkPrice:      mark,
		UnrealizedPnL:  pnl,
		NotionalUSD:    current * mark,
		InSync:         math.Abs(drift) < 0.01,
		DryRun:         dryRun,
		Available:      available,
		Note:           note,
	}
	return h
}

// geometricBrownianDemo mirrors the datasource GBM generator for the tracker's
// own use (kept local so the package has no test-only export coupling).
func geometricBrownianDemo(rng *rand.Rand, p0, annualVol float64, n int) []float64 {
	const hoursPerYear = 24 * 365
	dt := 1.0 / hoursPerYear
	sigmaStep := annualVol * math.Sqrt(dt)
	closes := make([]float64, 0, n)
	p := p0
	for i := 0; i < n; i++ {
		p *= math.Exp(-0.5*sigmaStep*sigmaStep + rng.NormFloat64()*sigmaStep)
		closes = append(closes, p)
	}
	return closes
}

func (t *DemoTracker) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	return nil
}
