package position

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/hedger"
	"github.com/filipekdick/lp-tracker-go/internal/lp"
)

// poolReader is the slice of *lp.Reader the tracker needs (eases testing).
type poolReader interface {
	ReadPosition(tokenID int64) (lp.PositionReport, error)
}

// poolPricer fetches a single pool's market data by address.
type poolPricer interface {
	PoolByAddress(ctx context.Context, chain datasource.Chain, address string) (datasource.RawPool, error)
}

// LiveTracker reads one or more positions from chain, prices their pools via
// GeckoTerminal, pulls implied vol from Deribit and reads/syncs the matching
// Binance shorts. When several token IDs are tracked it sums their volatile-leg
// exposure by asset (folding synthetics together) and hedges each asset with a
// single short.
type LiveTracker struct {
	reader   poolReader
	pricer   poolPricer
	iv       datasource.ImpliedVolSource
	bn       *binance.Client
	tokenIDs []int64
	chain    datasource.Chain
	dryRun   bool
	strategy *hedger.Strategy

	mu           sync.Mutex
	initialState *Snapshot
	history      []Snapshot
	// feeLedgers banks collected LP fees across harvests per tracked position
	// (token units), keyed by token ID, so a portfolio of positions keeps each
	// leg's harvest accounting separate.
	feeLedgers map[int64]*FeeLedger
}

// NewLiveTracker wires the live data sources. bn may be nil, in which case the
// hedge is reported as advisory (target only, no live short). tokenIDs lists
// every LP position to track; with more than one the hedge is aggregated.
func NewLiveTracker(
	reader poolReader,
	pricer poolPricer,
	iv datasource.ImpliedVolSource,
	bn *binance.Client,
	tokenIDs []int64,
	chain datasource.Chain,
	dryRun bool,
	strategy *hedger.Strategy,
) *LiveTracker {
	return &LiveTracker{
		reader:     reader,
		pricer:     pricer,
		iv:         iv,
		bn:         bn,
		tokenIDs:   tokenIDs,
		chain:      chain,
		dryRun:     dryRun,
		strategy:   strategy,
		feeLedgers: map[int64]*FeeLedger{},
	}
}

func (t *LiveTracker) Name() string { return "live" }

// trackedLeg bundles one position's on-chain report and priced pool so the
// aggregation, valuation and fee ledger can all reuse a single read.
type trackedLeg struct {
	report lp.PositionReport
	rp     datasource.RawPool
	priced bool
	price0 float64
	price1 float64
	value  float64
	leg    PositionLeg
}

// Track reads every tracked position, aggregates the exposure across them and
// reports a single simplified hedge per asset.
func (t *LiveTracker) Track(ctx context.Context) (TrackedPosition, error) {
	if len(t.tokenIDs) == 0 {
		return TrackedPosition{}, fmt.Errorf("no token IDs configured to track")
	}

	var legs []trackedLeg
	var firstErr error
	for _, id := range t.tokenIDs {
		report, err := t.reader.ReadPosition(id)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("reading position %d: %w", id, err)
			}
			log.Printf("[tracker] reading position %d failed: %v", id, err)
			continue
		}
		legs = append(legs, t.priceLeg(ctx, report))
	}
	// Every position failed to read: surface the first error.
	if len(legs) == 0 {
		return TrackedPosition{}, firstErr
	}

	tp := buildPortfolio(legs)
	tp.Source = "live"
	tp.UpdatedAt = time.Now()

	// Aggregate exposure across all positions and build one hedge per asset.
	tp.Exposures = aggregateExposures(tp.Positions)
	tp.Hedges = t.hedges(ctx, tp.Exposures)
	if len(tp.Hedges) > 0 {
		tp.Hedge = tp.Hedges[0]
	}

	if t.bn != nil {
		if open, err := t.bn.GetOpenPositions(ctx); err == nil {
			tp.OpenShorts = open
		}
		if orders, err := t.bn.GetAllOpenOrders(ctx); err == nil {
			tp.OpenLimitOrders = orders
		}
	}

	// Sum of the open shorts' unrealized PnL — the live, resettable part of the
	// hedge PnL.
	hedgeUnrealized := 0.0
	for _, h := range tp.Hedges {
		hedgeUnrealized += h.UnrealizedPnL
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Re-derive the cumulative hedge income ledger (realized PnL, funding,
	// commissions) from Binance since inception for every aggregated perp symbol.
	var ledger binance.IncomeLedger
	if t.bn != nil {
		inceptionMs := snap0(t.initialState).Timestamp.UnixMilli()
		if t.initialState == nil {
			inceptionMs = tp.UpdatedAt.UnixMilli() // first poll: window is empty → zero ledger
		}
		if syms := hedgeSymbols(tp.Hedges); len(syms) > 0 {
			if l, err := t.bn.GetHedgeIncome(ctx, syms, inceptionMs); err == nil {
				ledger = l
			} else {
				log.Printf("[hedger] income history unavailable: %v", err)
			}
		}
	}

	// Update the per-position LP fee ledgers in TOKEN UNITS (harvest detection)
	// and sum across the portfolio, priced at each pool's current prices.
	feesToCollect, feesTotal := 0.0, 0.0
	for _, l := range legs {
		led := t.feeLedgers[l.report.TokenID]
		if led == nil {
			led = &FeeLedger{}
			t.feeLedgers[l.report.TokenID] = led
		}
		led.Update(l.report.UncollectedFees0, l.report.UncollectedFees1)
		feesToCollect += led.ToCollectUSD(l.price0, l.price1)
		feesTotal += led.TotalUSD(l.price0, l.price1)
	}

	// Snapshot for history (portfolio-level).
	snap := Snapshot{
		Timestamp:           tp.UpdatedAt,
		Price0:              tp.Price0,
		Price1:              tp.Price1,
		Amount0:             tp.Amount0,
		Amount1:             tp.Amount1,
		ValueUSD:            tp.ValueUSD,
		HedgePnL:            hedgeUnrealized,
		HedgeRealizedPnL:    ledger.RealizedPnL,
		HedgeFundingUSD:     ledger.Funding,
		HedgeCommissionsUSD: ledger.Commissions,
		FeesUSD:             feesTotal,
		NetPnL:              0,
	}

	tp.HedgeRealizedPnL = ledger.RealizedPnL
	tp.HedgeFundingUSD = ledger.Funding
	tp.HedgeCommissionsUSD = ledger.Commissions
	tp.HedgeIncomePartial = ledger.Partial
	tp.FeesToCollectUSD = feesToCollect
	tp.FeesTotalUSD = feesTotal

	anyPriced := tp.ValueUSD > 0 || tp.Error == ""
	if t.initialState == nil && anyPriced {
		t.initialState = &Snapshot{
			Timestamp:           snap.Timestamp,
			Price0:              snap.Price0,
			Price1:              snap.Price1,
			Amount0:             snap.Amount0,
			Amount1:             snap.Amount1,
			ValueUSD:            snap.ValueUSD,
			HedgePnL:            snap.HedgePnL,
			HedgeRealizedPnL:    snap.HedgeRealizedPnL,
			HedgeFundingUSD:     snap.HedgeFundingUSD,
			HedgeCommissionsUSD: snap.HedgeCommissionsUSD,
			FeesUSD:             snap.FeesUSD,
			NetPnL:              0,
		}
	}

	if t.initialState != nil {
		comps := PnLComponents{
			LPChange:        snap.ValueUSD - t.initialState.ValueUSD,
			HedgeUnrealized: snap.HedgePnL - t.initialState.HedgePnL,
			HedgeRealized:   ledger.RealizedPnL,
			Funding:         ledger.Funding,
			Commissions:     ledger.Commissions,
			LPFees:          snap.FeesUSD - t.initialState.FeesUSD,
		}
		snap.NetPnL = comps.NetPnL()

		elapsed := snap.Timestamp.Sub(t.initialState.Timestamp)
		tp.Analysis.PositionFeeAPR = analyzer.PositionFeeAPR(comps.LPFees, tp.ValueUSD, elapsed)

		t.history = append(t.history, snap)
		if len(t.history) > 1000 {
			t.history = t.history[len(t.history)-1000:]
		}
		tp.InitialState = t.initialState
		tp.History = t.history
	}

	return tp, nil
}

// priceLeg prices one position's pool and runs the fee-vs-volatility model,
// returning a trackedLeg with the per-position PositionLeg filled in. A pricing
// failure is non-fatal: the leg is still returned with whatever on-chain data is
// available and an Error note.
func (t *LiveTracker) priceLeg(ctx context.Context, report lp.PositionReport) trackedLeg {
	tl := trackedLeg{report: report}
	leg := PositionLeg{
		TokenID:          report.TokenID,
		Chain:            t.chain.Display,
		ChainSlug:        t.chain.Slug,
		ChainKind:        t.chain.Kind,
		PoolAddress:      report.PoolAddress,
		PoolName:         fmt.Sprintf("%s / %s", report.Symbol0, report.Symbol1),
		Symbol0:          report.Symbol0,
		Symbol1:          report.Symbol1,
		Amount0:          report.Amount0,
		Amount1:          report.Amount1,
		TickLower:        report.TickLower,
		TickUpper:        report.TickUpper,
		TickNow:          report.TickNow,
		InRange:          report.InRange,
		Decimals0:        report.Decimals0,
		Decimals1:        report.Decimals1,
		UncollectedFees0: report.UncollectedFees0,
		UncollectedFees1: report.UncollectedFees1,
	}

	rp, perr := t.pricer.PoolByAddress(ctx, t.chain, report.PoolAddress)
	if perr != nil {
		leg.Error = fmt.Sprintf("pool pricing unavailable: %v", perr)
		tl.leg = withRangePrices(leg)
		return tl
	}

	tl.rp = rp
	tl.priced = true
	tl.price0 = legPrice(report.Symbol0, rp)
	tl.price1 = legPrice(report.Symbol1, rp)

	// Prefer Binance mark prices for hedged legs when a client is present.
	if t.bn != nil {
		if perp, ok := perpForSymbol(report.Symbol0); ok {
			if price, err := t.bn.GetMarkPrice(ctx, perp); err == nil && price > 0 {
				tl.price0 = price
			}
		}
		if perp, ok := perpForSymbol(report.Symbol1); ok {
			if price, err := t.bn.GetMarkPrice(ctx, perp); err == nil && price > 0 {
				tl.price1 = price
			}
		}
	}

	leg.Protocol = rp.Protocol
	leg.DEX = rp.DEX
	leg.FeeTier = rp.FeeTier
	leg.TVLUSD = rp.TVLUSD
	leg.Volume24hUSD = rp.Volume24hUSD
	leg.Analysis = runAnalysis(ctx, rp, t.iv)
	leg.Price0 = tl.price0
	leg.Price1 = tl.price1
	leg.ValueUSD = report.Amount0*tl.price0 + report.Amount1*tl.price1
	tl.value = leg.ValueUSD
	tl.leg = withRangePrices(leg)
	return tl
}

// withRangePrices fills a PositionLeg's notional range-price fields from its
// ticks + decimals (display-only, no network/chain access).
func withRangePrices(leg PositionLeg) PositionLeg {
	tp := TrackedPosition{
		TickLower: leg.TickLower, TickUpper: leg.TickUpper, TickNow: leg.TickNow,
		Decimals0: leg.Decimals0, Decimals1: leg.Decimals1,
	}
	tp.fillRangePrices()
	leg.RangeLowerPrice = tp.RangeLowerPrice
	leg.RangeUpperPrice = tp.RangeUpperPrice
	leg.RangeCurrentPrice = tp.RangeCurrentPrice
	leg.RangePositionPct = tp.RangePositionPct
	return leg
}

// buildPortfolio assembles the aggregated TrackedPosition from the tracked legs.
// The top-level pool fields mirror the first (primary) position for backward
// compatibility with the single-position dashboard cards, while ValueUSD,
// Positions and (later) Exposures describe the whole portfolio.
func buildPortfolio(legs []trackedLeg) TrackedPosition {
	tp := TrackedPosition{}
	for i, l := range legs {
		tp.Positions = append(tp.Positions, l.leg)
		tp.ValueUSD += l.value
		if i == 0 {
			primary := l.leg
			tp.TokenID = primary.TokenID
			tp.Chain = primary.Chain
			tp.ChainSlug = primary.ChainSlug
			tp.ChainKind = primary.ChainKind
			tp.Protocol = primary.Protocol
			tp.DEX = primary.DEX
			tp.PoolAddress = primary.PoolAddress
			tp.PoolName = primary.PoolName
			tp.Symbol0 = primary.Symbol0
			tp.Symbol1 = primary.Symbol1
			tp.Amount0 = primary.Amount0
			tp.Amount1 = primary.Amount1
			tp.TickLower = primary.TickLower
			tp.TickUpper = primary.TickUpper
			tp.TickNow = primary.TickNow
			tp.InRange = primary.InRange
			tp.Decimals0 = primary.Decimals0
			tp.Decimals1 = primary.Decimals1
			tp.RangeLowerPrice = primary.RangeLowerPrice
			tp.RangeUpperPrice = primary.RangeUpperPrice
			tp.RangeCurrentPrice = primary.RangeCurrentPrice
			tp.RangePositionPct = primary.RangePositionPct
			tp.UncollectedFees0 = primary.UncollectedFees0
			tp.UncollectedFees1 = primary.UncollectedFees1
			tp.TokensOwed0 = primary.UncollectedFees0
			tp.TokensOwed1 = primary.UncollectedFees1
			tp.Price0 = primary.Price0
			tp.Price1 = primary.Price1
			tp.FeeTier = primary.FeeTier
			tp.TVLUSD = primary.TVLUSD
			tp.Volume24hUSD = primary.Volume24hUSD
			tp.Analysis = primary.Analysis
			tp.Error = primary.Error
		}
	}
	return tp
}

// snap0 returns a zero-value Snapshot for a nil pointer, so callers can read a
// field without a nil check.
func snap0(s *Snapshot) Snapshot {
	if s == nil {
		return Snapshot{}
	}
	return *s
}

// hedgeSymbols collects the distinct perp symbols of the available hedges, for
// the income-history fetch.
func hedgeSymbols(hedges []Hedge) []string {
	seen := map[string]bool{}
	var syms []string
	for _, h := range hedges {
		if h.Symbol == "" || seen[h.Symbol] {
			continue
		}
		seen[h.Symbol] = true
		syms = append(syms, h.Symbol)
	}
	return syms
}

// hedges builds one simplified short per aggregated asset exposure and reads (or
// syncs, when not dry-run) the matching Binance position.
func (t *LiveTracker) hedges(ctx context.Context, exposures []AssetExposure) []Hedge {
	if len(exposures) == 0 {
		return []Hedge{{
			Available: false,
			Note:      "no hedgeable (perp-listed) leg across tracked positions",
		}}
	}

	var hedges []Hedge
	for _, ex := range exposures {
		h := Hedge{
			Venue:          "Binance Futures",
			Symbol:         ex.Perp,
			ExposureSymbol: ex.DisplaySymbol,
			LPExposure:     ex.Amount,
			TargetShort:    ex.Amount,
			DryRun:         t.dryRun,
			Note:           sourcesNote(ex),
		}

		if t.bn == nil {
			h.Available = false
			h.Note = "advisory only — no Binance credentials configured · " + h.Note
			hedges = append(hedges, h)
			continue
		}

		// Sync the single aggregate short to the summed exposure (respects dryRun).
		if err := t.bn.SyncShort(ctx, ex.Perp, ex.Amount, 0.001, t.dryRun, t.strategy); err != nil {
			log.Printf("[hedger] SyncShort failed for %s: %v", ex.Perp, err)
		}

		size, err := t.bn.GetPositionSize(ctx, ex.Perp)
		if err != nil {
			h.Available = false
			h.Note = fmt.Sprintf("could not read Binance position: %v", err)
			hedges = append(hedges, h)
			continue
		}
		current := 0.0
		if size < 0 {
			current = -size
		}
		h.CurrentShort = current
		h.Drift = h.TargetShort - current
		h.InSync = absf(h.Drift) < 0.01
		h.Available = true
		if t.dryRun {
			h.Note = "live read · hedge sync is dry-run · " + h.Note
		} else {
			h.Note = "live · " + h.Note
		}

		if open, err := t.bn.GetOpenPositions(ctx); err == nil {
			for _, p := range open {
				if p.Symbol == ex.Perp {
					h.EntryPrice = p.EntryPrice
					h.MarkPrice = p.MarkPrice
					h.UnrealizedPnL = p.UnrealizedPnL
					h.NotionalUSD = absf(p.Size) * p.MarkPrice
					break
				}
			}
		}
		hedges = append(hedges, h)
	}
	return hedges
}

// sourcesNote summarises which positions contribute to an aggregated exposure,
// e.g. "summed from 2 positions (WETH, wstETH)".
func sourcesNote(ex AssetExposure) string {
	if len(ex.Sources) <= 1 {
		return "single position"
	}
	syms := map[string]bool{}
	var order []string
	for _, s := range ex.Sources {
		if !syms[s.Symbol] {
			syms[s.Symbol] = true
			order = append(order, s.Symbol)
		}
	}
	return fmt.Sprintf("summed from %d positions (%s)", len(ex.Sources), strings.Join(order, ", "))
}

// legPrice returns the USD price of one token symbol from the pool's reported
// base/quote prices, matching by symbol and defaulting stablecoins to $1.
func legPrice(sym string, rp datasource.RawPool) float64 {
	switch strings.ToUpper(sym) {
	case strings.ToUpper(rp.BaseSymbol):
		return rp.PriceUSD
	case strings.ToUpper(rp.QuoteSymbol):
		return rp.QuotePriceUSD
	}
	if stableSymbols[strings.ToUpper(sym)] {
		return 1
	}
	return 0
}

func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func (t *LiveTracker) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	if t.bn == nil {
		return fmt.Errorf("binance client not connected")
	}
	return t.bn.CancelOrder(ctx, symbol, orderID)
}
