package position

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
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
	reader  poolReader
	pricer  poolPricer
	iv      datasource.ImpliedVolSource
	bn      *binance.Client
	tokenID int64
	chain   datasource.Chain
	dryRun  bool
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
) *LiveTracker {
	return &LiveTracker{
		reader:  reader,
		pricer:  pricer,
		iv:      iv,
		bn:      bn,
		tokenID: tokenID,
		chain:   chain,
		dryRun:  dryRun,
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
		TokenID:     t.tokenID,
		Chain:       t.chain.Display,
		ChainSlug:   t.chain.Slug,
		ChainKind:   t.chain.Kind,
		PoolAddress: report.PoolAddress,
		PoolName:    fmt.Sprintf("%s / %s", report.Symbol0, report.Symbol1),
		Symbol0:     report.Symbol0,
		Symbol1:     report.Symbol1,
		Amount0:     report.Amount0,
		Amount1:     report.Amount1,
		TickLower:   report.TickLower,
		TickUpper:   report.TickUpper,
		TickNow:     report.TickNow,
		InRange:     report.InRange,
		Source:      "live",
		UpdatedAt:   time.Now(),
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
	}

	tp.Hedge = t.hedge(ctx, report)
	return tp, nil
}

// hedge determines the perp short that offsets the position's volatile leg and
// reads the current Binance position when credentials are configured.
func (t *LiveTracker) hedge(ctx context.Context, report lp.PositionReport) Hedge {
	asset, amount, perp, ok := hedgeLeg(report.Symbol0, report.Symbol1, report.Amount0, report.Amount1)
	if !ok {
		return Hedge{
			Available: false,
			Note:      "no hedgeable (perp-listed) leg in this pool",
		}
	}

	h := Hedge{
		Venue:          "Binance Futures",
		Symbol:         perp,
		ExposureSymbol: asset,
		LPExposure:     amount,
		TargetShort:    amount,
		DryRun:         t.dryRun,
	}

	if t.bn == nil {
		h.Available = false
		h.Note = "advisory only — no Binance credentials configured"
		return h
	}

	size, err := t.bn.GetPositionSize(ctx, perp)
	if err != nil {
		h.Available = false
		h.Note = fmt.Sprintf("could not read Binance position: %v", err)
		return h
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
			if p.Symbol == perp {
				h.EntryPrice = p.EntryPrice
				h.MarkPrice = p.MarkPrice
				h.UnrealizedPnL = p.UnrealizedPnL
				h.NotionalUSD = absf(p.Size) * p.MarkPrice
				break
			}
		}
	}
	return h
}

// positionValueUSD prices both legs of the position using the pool's reported
// base/quote USD prices, matching by symbol.
func positionValueUSD(report lp.PositionReport, rp datasource.RawPool) float64 {
	price := func(sym string) float64 {
		switch strings.ToUpper(sym) {
		case strings.ToUpper(rp.BaseSymbol):
			return rp.PriceUSD
		case strings.ToUpper(rp.QuoteSymbol):
			return rp.QuotePriceUSD
		}
		// Stablecoins default to $1 when unmatched.
		if stableSymbols[strings.ToUpper(sym)] {
			return 1
		}
		return 0
	}
	return report.Amount0*price(report.Symbol0) + report.Amount1*price(report.Symbol1)
}

func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
