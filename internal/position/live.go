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

// LiveTracker reads a position from chain, prices its pool via GeckoTerminal,
// pulls implied vol from Deribit and reads the matching Binance short.
type LiveTracker struct {
	reader   poolReader
	pricer   poolPricer
	iv       datasource.ImpliedVolSource
	bn       *binance.Client
	tokenID  int64
	chain    datasource.Chain
	dryRun   bool
	strategy *hedger.Strategy

	mu           sync.Mutex
	initialState *Snapshot
	history      []Snapshot
	feeLedger    FeeLedger // banks collected LP fees across harvests (token units)
}

// NewLiveTracker wires the live data sources. bn may be nil, in which case the
// hedge is reported as advisory (target only, no live short).
func NewLiveTracker(
	reader poolReader,
	pricer poolPricer,
	iv datasource.ImpliedVolSource,
	bn *binance.Client,
	tokenID int64,
	chain datasource.Chain,
	dryRun bool,
	strategy *hedger.Strategy,
) *LiveTracker {
	return &LiveTracker{
		reader:   reader,
		pricer:   pricer,
		iv:       iv,
		bn:       bn,
		tokenID:  tokenID,
		chain:    chain,
		dryRun:   dryRun,
		strategy: strategy,
	}
}

func (t *LiveTracker) Name() string { return "live" }

// Track reads the current position, pool analysis and hedge state.
func (t *LiveTracker) Track(ctx context.Context) (TrackedPosition, error) {
	report, err := t.reader.ReadPosition(t.tokenID)
	if err != nil {
		return TrackedPosition{}, fmt.Errorf("reading position %d: %w", t.tokenID, err)
	}

	tp := TrackedPosition{
		TokenID:          t.tokenID,
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
		TokensOwed0:      report.UncollectedFees0, // alias (back-compat)
		TokensOwed1:      report.UncollectedFees1,
		Source:           "live",
		UpdatedAt:        time.Now(),
	}

	// Price the pool and run the fee-vs-volatility model. A pricing failure is
	// non-fatal: we still report the position and hedge.
	rp, perr := t.pricer.PoolByAddress(ctx, t.chain, report.PoolAddress)
	if perr != nil {
		tp.Error = fmt.Sprintf("pool pricing unavailable: %v", perr)
	} else {
		tp.Protocol = rp.Protocol
		tp.DEX = rp.DEX
		tp.FeeTier = rp.FeeTier
		tp.TVLUSD = rp.TVLUSD
		tp.Volume24hUSD = rp.Volume24hUSD
		tp.Analysis = runAnalysis(ctx, rp, t.iv)
		tp.ValueUSD = positionValueUSD(report, rp)
		tp.Price0 = legPrice(report.Symbol0, rp)
		tp.Price1 = legPrice(report.Symbol1, rp)

		if t.bn != nil {
			if perp, ok := hedgeFutures[datasource.NormalizeSymbol(report.Symbol0)]; ok {
				if price, err := t.bn.GetMarkPrice(ctx, perp); err == nil && price > 0 {
					tp.Price0 = price
				}
			}
			if perp, ok := hedgeFutures[datasource.NormalizeSymbol(report.Symbol1)]; ok {
				if price, err := t.bn.GetMarkPrice(ctx, perp); err == nil && price > 0 {
					tp.Price1 = price
				}
			}
		}
	}

	tp.fillRangePrices()

	tp.Hedges = t.hedges(ctx, report)
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

	price0, price1 := 0.0, 0.0
	if rp.Address != "" {
		price0 = legPrice(report.Symbol0, rp)
		price1 = legPrice(report.Symbol1, rp)
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
	// commissions) from Binance since inception. The income endpoint is the
	// source of truth and re-fetching is robust, so we do not persist a running
	// total — this relies on inception falling inside binance.IncomeLookback; if
	// it predates that window the ledger comes back flagged Partial. Persisting
	// across restarts is a separate later task.
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

	// Update the LP fee ledger in TOKEN UNITS so a harvest (a leg's uncollected
	// token amount falling) is detected without being fooled by a price-only drop
	// in fee USD value. The cumulative total = banked collected + current
	// uncollected, priced at current prices.
	t.feeLedger.Update(report.UncollectedFees0, report.UncollectedFees1)
	feesToCollect := t.feeLedger.ToCollectUSD(price0, price1)
	feesTotal := t.feeLedger.TotalUSD(price0, price1)

	// Snapshot for history.
	snap := Snapshot{
		Timestamp:           tp.UpdatedAt,
		Price0:              price0,
		Price1:              price1,
		Amount0:             report.Amount0,
		Amount1:             report.Amount1,
		ValueUSD:            tp.ValueUSD,
		HedgePnL:            hedgeUnrealized,
		HedgeRealizedPnL:    ledger.RealizedPnL,
		HedgeFundingUSD:     ledger.Funding,
		HedgeCommissionsUSD: ledger.Commissions,
		FeesUSD:             feesTotal, // cumulative collected + uncollected
		NetPnL:              0,
	}

	// Surface the ledgers on the tracked position for the API/dashboard.
	tp.HedgeRealizedPnL = ledger.RealizedPnL
	tp.HedgeFundingUSD = ledger.Funding
	tp.HedgeCommissionsUSD = ledger.Commissions
	tp.HedgeIncomePartial = ledger.Partial
	tp.FeesToCollectUSD = feesToCollect
	tp.FeesTotalUSD = feesTotal

	if t.initialState == nil && tp.Error == "" && rp.Address != "" {
		// First successful scan, capture baseline.
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
			NetPnL:              0, // Initial net PnL is zero
		}
	}

	if t.initialState != nil {
		// Strategy net PnL = the *change* since inception, summing every term with
		// its documented sign (see PnLComponents.NetPnL):
		//   + LP value change   (current LP value − inception LP value)
		//   + hedge unrealized  (open short PnL, baselined so the curve starts at 0)
		//   + hedge realized    (cumulative since inception — already ~0 at start
		//                        because the income window opens at inception)
		//   + funding           (cumulative, signed: + when the short receives)
		//   − commissions       (cumulative paid cost)
		//   + LP fees           (cumulative collected + uncollected, baselined)
		// The hedge unrealized and LP fees are baselined against inception; the
		// income terms are inception-relative by construction (the income fetch
		// starts at inception), so they are not baselined again.
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

		// Optional: limit history size
		if len(t.history) > 1000 {
			t.history = t.history[len(t.history)-1000:]
		}

		tp.InitialState = t.initialState
		tp.History = t.history
	}

	return tp, nil
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

// hedges determines the perp shorts that offset the position's volatile legs and
// reads the current Binance positions when credentials are configured.
// It also places/syncs orders if credentials are configured and dry-run is false.
func (t *LiveTracker) hedges(ctx context.Context, report lp.PositionReport) []Hedge {
	legs := hedgeLegs(report.Symbol0, report.Symbol1, report.Amount0, report.Amount1)
	if len(legs) == 0 {
		return []Hedge{{
			Available: false,
			Note:      "no hedgeable (perp-listed) leg in this pool",
		}}
	}

	var hedges []Hedge
	for _, leg := range legs {
		h := Hedge{
			Venue:          "Binance Futures",
			Symbol:         leg.Perp,
			ExposureSymbol: leg.Asset,
			LPExposure:     leg.Amount,
			TargetShort:    leg.Amount,
			DryRun:         t.dryRun,
		}

		if t.bn == nil {
			h.Available = false
			h.Note = "advisory only — no Binance credentials configured"
			hedges = append(hedges, h)
			continue
		}

		// Perform the actual hedge sync if keys are present (SyncShort respects dryRun)
		err := t.bn.SyncShort(ctx, leg.Perp, leg.Amount, 0.001, t.dryRun, t.strategy)
		if err != nil {
			log.Printf("[hedger] SyncShort failed for %s: %v", leg.Perp, err)
		}

		size, err := t.bn.GetPositionSize(ctx, leg.Perp)
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
		h.Note = "live"
		if t.dryRun {
			h.Note = "live read · hedge sync is dry-run"
		}

		// Enrich with entry/mark/PnL from the open positions list.
		if open, err := t.bn.GetOpenPositions(ctx); err == nil {
			for _, p := range open {
				if p.Symbol == leg.Perp {
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

// legPrices returns the current USD prices of the position's two legs.
func legPrices(report lp.PositionReport, rp datasource.RawPool) (float64, float64) {
	return legPrice(report.Symbol0, rp), legPrice(report.Symbol1, rp)
}

// positionValueUSD prices both legs of the position using the pool's reported
// base/quote USD prices, matching by symbol.
func positionValueUSD(report lp.PositionReport, rp datasource.RawPool) float64 {
	return report.Amount0*legPrice(report.Symbol0, rp) + report.Amount1*legPrice(report.Symbol1, rp)
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
