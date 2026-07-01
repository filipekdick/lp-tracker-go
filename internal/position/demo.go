package position

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
)

// defaultDemoTokenID is used when a demo tracker is built with no IDs.
const defaultDemoTokenID = 71002035

// DemoTracker synthesises a realistic Aerodrome portfolio on Base — one
// ETH-family position per tracked token ID, all folding into a single ETH perp
// short — so the dashboard runs offline. The tracked set is mutable at runtime
// (SetTokenIDs), so the frontend's "edit tracked positions" button works in demo
// mode too.
type DemoTracker struct {
	mu       sync.Mutex
	tokenIDs []int64
	iv       datasource.ImpliedVolSource
}

// NewDemoTracker builds a demo tracker for the given token IDs (one synthetic
// position each). With no IDs it falls back to the built-in default.
func NewDemoTracker(tokenIDs ...int64) *DemoTracker {
	ids := dedupePositive(tokenIDs)
	if len(ids) == 0 {
		ids = []int64{defaultDemoTokenID}
	}
	return &DemoTracker{tokenIDs: ids, iv: datasource.DemoImpliedVol{}}
}

func (t *DemoTracker) Name() string { return "demo" }

// TokenIDs returns the currently tracked token IDs.
func (t *DemoTracker) TokenIDs() []int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]int64(nil), t.tokenIDs...)
}

// SetTokenIDs replaces the tracked token IDs (invalid/duplicate/non-positive
// entries are dropped). An empty result falls back to the built-in default.
func (t *DemoTracker) SetTokenIDs(ids []int64) {
	clean := dedupePositive(ids)
	if len(clean) == 0 {
		clean = []int64{defaultDemoTokenID}
	}
	t.mu.Lock()
	t.tokenIDs = clean
	t.mu.Unlock()
}

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

	// 14 days of hourly WETH OHLC ending near the live mark (one window, used for
	// all volatility methods); the 7-day close tail drives the legacy default.
	ohlc := geometricBrownianDemoOHLC(rng, entryPrice, ethVol, 14*24)
	closes := closesTailDemo(ohlc, 7*24)
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
		OHLC:           ohlc,
		PeriodsPerYear: 24 * 365,
	}

	res := runAnalysis(ctx, rp, t.iv)

	perp, _ := resolvePerp("ETH") // ETHUSDC by default (USDC-preferred)

	// One synthetic position per tracked token ID, all ETH-family so they fold
	// into a single aggregated ETH short. The first is the rich WETH/USDC position
	// that drives the top-level cards and the history graph; the rest alternate
	// between another WETH/USDC and a wstETH/WETH (synthetic ETH) pool, so adding
	// token IDs from the frontend visibly grows the portfolio and the hedge.
	ids := t.TokenIDs()
	primaryLeg := PositionLeg{
		TokenID: ids[0], Chain: rp.Chain, ChainSlug: rp.ChainSlug, ChainKind: rp.ChainKind,
		Protocol: rp.Protocol, DEX: rp.DEX, PoolAddress: rp.Address, PoolName: rp.Name,
		Symbol0: "WETH", Symbol1: "USDC", Amount0: wethAmount, Amount1: usdcAmount,
		TickLower: -201_400, TickUpper: -199_800, TickNow: -200_600, InRange: true,
		Decimals0: 18, Decimals1: 6, ValueUSD: valueUSD, Price0: mark, Price1: 1,
		FeeTier: rp.FeeTier, TVLUSD: rp.TVLUSD, Volume24hUSD: rp.Volume24hUSD, Analysis: res,
	}
	positions := []PositionLeg{withRangePrices(primaryLeg)}
	totalETH := wethAmount
	totalValue := valueUSD
	for i := 1; i < len(ids); i++ {
		var leg PositionLeg
		if i%2 == 1 {
			// wstETH/WETH synthetic — ETH exposure via wstETH + WETH.
			wst := 1.0 + rng.Float64()*1.2
			w2 := 0.4 + rng.Float64()*0.8
			leg = PositionLeg{
				TokenID: ids[i], Chain: rp.Chain, ChainSlug: rp.ChainSlug, ChainKind: rp.ChainKind,
				Protocol: "Aerodrome", DEX: "aerodrome_slipstream",
				PoolAddress: "0x4d69971ccd4a636c403a3c1b00c85e99bb9b5606", PoolName: "wstETH / WETH",
				Symbol0: "wstETH", Symbol1: "WETH", Amount0: wst, Amount1: w2,
				TickLower: -2_000, TickUpper: 2_000, TickNow: 100, InRange: true,
				Decimals0: 18, Decimals1: 18, ValueUSD: (wst + w2) * mark, Price0: mark, Price1: mark,
				FeeTier: 0.0001, TVLUSD: 9_000_000, Volume24hUSD: 4_000_000, Analysis: res,
			}
			totalETH += wst + w2
		} else {
			// Another WETH/USDC position.
			w := 1.0 + rng.Float64()*2.0
			u := w * mark * (0.8 + rng.Float64()*0.3)
			leg = PositionLeg{
				TokenID: ids[i], Chain: rp.Chain, ChainSlug: rp.ChainSlug, ChainKind: rp.ChainKind,
				Protocol: rp.Protocol, DEX: rp.DEX, PoolAddress: rp.Address, PoolName: rp.Name,
				Symbol0: "WETH", Symbol1: "USDC", Amount0: w, Amount1: u,
				TickLower: -202_000, TickUpper: -199_000, TickNow: -200_600, InRange: true,
				Decimals0: 18, Decimals1: 6, ValueUSD: w*mark + u, Price0: mark, Price1: 1,
				FeeTier: rp.FeeTier, TVLUSD: rp.TVLUSD, Volume24hUSD: rp.Volume24hUSD, Analysis: res,
			}
			totalETH += w
		}
		positions = append(positions, withRangePrices(leg))
		totalValue += leg.ValueUSD
	}
	exposures := aggregateExposures(positions)

	// Aggregate ETH exposure across all positions → a single simplified short,
	// deliberately a touch out of sync.
	current := totalETH - (0.05 + rng.Float64()*0.25)
	hedge := buildHedge("WETH", perp, totalETH, current, entryPrice, mark, true, true,
		"Binance Futures (testnet) · demo · "+sourcesNote(exposures[0]))

	tp := TrackedPosition{
		TokenID:     ids[0],
		Chain:       rp.Chain,
		ChainSlug:   rp.ChainSlug,
		ChainKind:   rp.ChainKind,
		Protocol:    rp.Protocol,
		DEX:         rp.DEX,
		PoolAddress: rp.Address,
		PoolName:    rp.Name,
		Symbol0:     "WETH",
		Symbol1:     "USDC",
		Amount0:     wethAmount,
		Amount1:     usdcAmount,
		TickLower:   -201_400,
		TickUpper:   -199_800,
		TickNow:     -200_600,
		InRange:     true,
		Decimals0:   18, // WETH
		Decimals1:   6,  // USDC
		// Synthetic live claimable fees.
		UncollectedFees0: 0.0042 + rng.Float64()*0.002,
		UncollectedFees1: 8 + rng.Float64()*6,
		TokensOwed0:      0.0042 + rng.Float64()*0.002,
		TokensOwed1:      8 + rng.Float64()*6,
		ValueUSD:         totalValue,
		Price0:           mark,
		Price1:           1,
		FeeTier:          rp.FeeTier,
		TVLUSD:           rp.TVLUSD,
		Volume24hUSD:     rp.Volume24hUSD,
		Analysis:         res,
		Positions:        positions,
		Exposures:        exposures,
		Hedges:           []Hedge{hedge},
		Hedge:            hedge,
		Source:           "demo",
		UpdatedAt:        time.Now(),
	}

	tp.fillRangePrices()

	// Synthetic hedge income ledger (cumulative USD since inception) so the new
	// realized / funding / commission fields render with no network. Signs match
	// the live path: realized and funding signed, commissions a positive cost.
	realized := 35 + rng.Float64()*20 // banked at past rebalances
	funding := 8 + rng.Float64()*6    // net funding the short received (positive)
	commissions := 4 + rng.Float64()*3
	tp.HedgeRealizedPnL = realized
	tp.HedgeFundingUSD = funding
	tp.HedgeCommissionsUSD = commissions
	tp.HedgeIncomePartial = false

	// Synthetic LP fee ledger. The current uncollected fees are "fees to collect";
	// the cumulative total adds a banked "collected" amount from a past harvest,
	// so the two fields differ — exactly what a real harvest produces.
	uncollectedUSD := tp.UncollectedFees0*mark + tp.UncollectedFees1
	collectedUSD := 50 + rng.Float64()*40 // banked by an earlier harvest
	tp.FeesToCollectUSD = uncollectedUSD
	tp.FeesTotalUSD = collectedUSD + uncollectedUSD

	// Synthetic open futures positions so the dashboard's "Open futures shorts"
	// table has rows to show. The ETH leg mirrors the hedge above; the others
	// give the table some variety.
	tp.OpenShorts = demoOpenShorts(rng, current, entryPrice, mark)

	// Synthetic strategy history so the Net-PnL/fees graph has a timeline to
	// draw before any live scans land. Live tracking builds this incrementally;
	// here we backfill a believable few hours in one shot.
	tp.InitialState, tp.History = demoHistory(rng, closes, wethAmount, usdcAmount, current, entryPrice,
		realized, funding, commissions, tp.FeesTotalUSD)

	// Synthetic open limit orders so the "Open limit orders" table and its
	// Cancel button are exercisable without live credentials.
	tp.OpenLimitOrders = []binance.LimitOrder{
		{Symbol: "ETHUSDT", OrderID: 9900667125, Side: "SELL", Price: mark * 1.001, OrigQty: 0.057, ExecutedQty: 0},
		{Symbol: "VVVUSDT", OrderID: 9900667126, Side: "BUY", Price: 15.077, OrigQty: 1.2, ExecutedQty: 0.3},
	}

	return tp, nil
}

// demoOpenShorts builds a handful of synthetic Binance positions for the
// open-shorts table. Short PnL follows size*(mark-entry) (size is negative).
func demoOpenShorts(rng *rand.Rand, ethShort, ethEntry, ethMark float64) []binance.Position {
	ethPerp, _ := resolvePerp("ETH")
	shorts := []binance.Position{
		{Symbol: ethPerp, Size: -ethShort, EntryPrice: ethEntry, MarkPrice: ethMark},
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
// graph paints a line on first load. NetPnL follows the full identity (see
// PnLComponents.NetPnL): LP value change + hedge unrealized + hedge realized +
// funding − commissions + cumulative LP fees. The cumulative income terms and
// fees ramp from zero at inception to their totals now, so the curve starts at
// zero like the baseline (NetPnL zero).
func demoHistory(rng *rand.Rand, closes []float64, wethAmount, usdcAmount, short, entry,
	realizedTotal, fundingTotal, commissionsTotal, feesTotalCum float64) (*Snapshot, []Snapshot) {
	const n = 24
	const step = 8 * time.Minute
	if len(closes) < n {
		return nil, nil
	}
	start := time.Now().Add(-(n - 1) * step)
	tail := closes[len(closes)-n:]
	// The position started with a bit more of the volatile leg and less of the
	// stable leg; as price drifted, some converted. This makes the demo's
	// impermanent-loss figure non-zero.
	amt0At := func(progress float64) float64 { return wethAmount * (1 + 0.05*(1-progress)) }
	amt1At := func(progress float64) float64 { return usdcAmount * (1 - 0.05*(1-progress)) }
	value0 := amt0At(0)*tail[0] + amt1At(0)
	hedge0 := short * (entry - tail[0]) // baseline hedge PnL at inception

	hist := make([]Snapshot, 0, n)
	for i := 0; i < n; i++ {
		progress := float64(i) / float64(n-1)
		price := tail[i]
		amt0, amt1 := amt0At(progress), amt1At(progress)
		value := amt0*price + amt1
		hedgePnL := short * (entry - price) // short gains as price falls
		// Cumulative terms accrue from zero at inception to their totals now.
		realized := realizedTotal * progress
		funding := fundingTotal * progress
		commissions := commissionsTotal * progress
		feesCum := feesTotalCum * progress // cumulative collected + uncollected
		comps := PnLComponents{
			LPChange:        value - value0,
			HedgeUnrealized: hedgePnL - hedge0,
			HedgeRealized:   realized,
			Funding:         funding,
			Commissions:     commissions,
			LPFees:          feesCum,
		}
		hist = append(hist, Snapshot{
			Timestamp:           start.Add(time.Duration(i) * step),
			Price0:              price,
			Price1:              1,
			Amount0:             amt0,
			Amount1:             amt1,
			ValueUSD:            value,
			HedgePnL:            hedgePnL,
			HedgeRealizedPnL:    realized,
			HedgeFundingUSD:     funding,
			HedgeCommissionsUSD: commissions,
			FeesUSD:             feesCum,
			NetPnL:              comps.NetPnL(),
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

// geometricBrownianDemoOHLC mirrors the datasource GBM OHLC generator for the
// tracker's own use (kept local so the package has no test-only export
// coupling). Bar highs/lows use the exact Brownian-bridge extremes so the
// range-based volatility method is unbiased on demo data too.
func geometricBrownianDemoOHLC(rng *rand.Rand, p0, annualVol float64, n int) []analyzer.OHLCV {
	const hoursPerYear = 24 * 365
	w := annualVol * annualVol / hoursPerYear
	sd := math.Sqrt(w)
	bars := make([]analyzer.OHLCV, 0, n)
	x := math.Log(p0)
	for i := 0; i < n; i++ {
		a := x
		b := x + (-0.5*w + rng.NormFloat64()*sd)
		qh := -(w / 2) * math.Log(rng.Float64())
		high := 0.5 * ((a + b) + math.Sqrt((a-b)*(a-b)+4*qh))
		ql := -(w / 2) * math.Log(rng.Float64())
		low := 0.5 * ((a + b) - math.Sqrt((a-b)*(a-b)+4*ql))
		bars = append(bars, analyzer.OHLCV{
			Time:   int64(i),
			Open:   math.Exp(a),
			High:   math.Exp(high),
			Low:    math.Exp(low),
			Close:  math.Exp(b),
			Volume: 1_000 + rng.Float64()*1_000_000,
		})
		x = b
	}
	return bars
}

// closesTailDemo returns the last n closes from a bar slice (oldest first).
func closesTailDemo(bars []analyzer.OHLCV, n int) []float64 {
	if n > len(bars) {
		n = len(bars)
	}
	out := make([]float64, 0, n)
	for _, b := range bars[len(bars)-n:] {
		out = append(out, b.Close)
	}
	return out
}

func (t *DemoTracker) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	return nil
}
