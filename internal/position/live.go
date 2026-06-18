package position

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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
		UncollectedFees0: report.UncollectedFees0,
		UncollectedFees1: report.UncollectedFees1,
		CollectedFees0:   report.CollectedFees0,
		CollectedFees1:   report.CollectedFees1,
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
		tp.Price0, tp.Price1 = legPrices(report, rp)
	}

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

	// Snapshot for history
	snap := Snapshot{
		Timestamp: tp.UpdatedAt,
		Price0:    0,
		Price1:    0,
		Amount0:   report.Amount0,
		Amount1:   report.Amount1,
		ValueUSD:  tp.ValueUSD,
		HedgePnL:  0,
		FeesUSD:   0,
		NetPnL:    0,
	}

	if rp.Address != "" {
		snap.Price0 = rp.PriceUSD
		snap.Price1 = rp.QuotePriceUSD
		snap.FeesUSD = report.UncollectedFees0*rp.PriceUSD + report.UncollectedFees1*rp.QuotePriceUSD
	}
	for _, h := range tp.Hedges {
		snap.HedgePnL += h.UnrealizedPnL
	}
	snap.NetPnL = snap.HedgePnL + snap.FeesUSD // simplified PnL

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.initialState == nil && tp.Error == "" && rp.Address != "" {
		// First successful scan, capture baseline
		t.initialState = &Snapshot{
			Timestamp: snap.Timestamp,
			Price0:    snap.Price0,
			Price1:    snap.Price1,
			Amount0:   snap.Amount0,
			Amount1:   snap.Amount1,
			ValueUSD:  snap.ValueUSD,
			HedgePnL:  snap.HedgePnL,
			FeesUSD:   snap.FeesUSD,
			NetPnL:    0, // Initial net PnL is zero
		}
	}

	if t.initialState != nil {
		// Net PnL of the strategy = the *change* since tracking started in LP
		// value + hedge PnL + accrued fees. Each leg is measured relative to the
		// baseline so the series starts at zero; using the absolute hedge PnL or
		// fee balance here would offset the whole curve by whatever those were
		// at inception.
		lpChange := snap.ValueUSD - t.initialState.ValueUSD
		hedgeChange := snap.HedgePnL - t.initialState.HedgePnL
		feesChange := snap.FeesUSD - t.initialState.FeesUSD
		snap.NetPnL = lpChange + hedgeChange + feesChange

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
