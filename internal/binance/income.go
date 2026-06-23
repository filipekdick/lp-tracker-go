package binance

import (
	"context"
	"fmt"
	"time"
)

// This file builds the hedge PnL ledger from the Binance USDⓈ-M futures
// income / transaction history endpoint (/fapi/v1/income). All three quantities
// the strategy needs — realized PnL banked at each rebalance, funding payments,
// and trading commissions — are income rows distinguished by their incomeType,
// so they come from one coherent fetch.
//
// Why this exists: the open-position unrealized PnL alone loses the realized
// PnL that left the position at each rebalance and ignores funding/commissions.
// Re-deriving the cumulative ledger from income history (the source of truth)
// on every poll keeps the strategy net PnL honest without persisting state.

// hedgeIncomeTypes are the income/transaction-history rows that make up the
// hedge ledger. REALIZED_PNL is the PnL booked when a leg closes, FUNDING_FEE is
// the periodic funding payment (signed), COMMISSION is the trading fee (a cost,
// reported by Binance as a negative income).
var hedgeIncomeTypes = []string{"REALIZED_PNL", "FUNDING_FEE", "COMMISSION"}

// incomePageLimit is the maximum number of rows the endpoint returns per call.
// We page until a call returns fewer than this.
const incomePageLimit = 1000

// IncomeLookback is how far back the USDⓈ-M income endpoint reliably serves
// data. If the strategy inception predates now-IncomeLookback we can only
// aggregate what is available, and the ledger is flagged Partial.
//
// ASSUMPTION: persisting a running total across process restarts is a separate
// later task. For this version we always re-derive from income history since
// inception, which is robust as long as inception falls inside this window.
const IncomeLookback = 90 * 24 * time.Hour

// IncomeRow is one income/transaction-history entry in our own simple form.
type IncomeRow struct {
	Symbol     string  // perp symbol, e.g. "ETHUSDT"
	Asset      string  // margin asset the row settled in: "USDT" or "USDC"
	IncomeType string  // REALIZED_PNL, FUNDING_FEE, or COMMISSION
	Income     float64 // signed amount in Asset units (Binance reports commission negative)
	Time       int64   // event time, unix ms
}

// IncomeLedger holds the cumulative USD sums derived from income history since
// strategy inception. Signs are documented per field; all three are in USD and
// independent of which stablecoin margin the short used.
type IncomeLedger struct {
	// RealizedPnL is the cumulative realized PnL of closed (rebalanced-out)
	// hedge legs, signed: positive when closing the short banked a gain.
	RealizedPnL float64
	// Funding is the cumulative funding payment, signed: positive when the short
	// receives funding, negative when it pays.
	Funding float64
	// Commissions is the cumulative trading commission PAID, stored as a positive
	// cost. It enters net PnL with a minus sign (it is subtracted), never added.
	Commissions float64
	// Partial is true when inception predates the income lookback window, so the
	// totals above are only over the available window, not the full lifetime.
	Partial bool
}

// incomeAssetUSD converts an income amount in its margin asset to USD. Both
// USDT and USDC are treated as ~1 USD, so the ledger is correct regardless of
// which stablecoin the short was collateralized in — and stays correct after a
// later switch from USDT- to USDC-margined futures. Any other asset is also
// treated as 1; in practice these futures only settle in USDT/USDC.
func incomeAssetUSD(_ string) float64 { return 1 }

// AggregateIncome folds income rows into the cumulative USD ledger. It is pure
// (no network) so the aggregation and its signs can be unit-tested directly.
func AggregateIncome(rows []IncomeRow) IncomeLedger {
	var l IncomeLedger
	for _, r := range rows {
		usd := r.Income * incomeAssetUSD(r.Asset)
		switch r.IncomeType {
		case "REALIZED_PNL":
			// Realized (closed legs) and unrealized (open position) are disjoint —
			// realized rows only appear once a leg is closed and no longer
			// contributes to the open position's unrealized PnL — so they add to the
			// complete hedge PnL without double-counting.
			l.RealizedPnL += usd
		case "FUNDING_FEE":
			// Signed: positive = short received funding, negative = short paid.
			l.Funding += usd
		case "COMMISSION":
			// Binance reports commission as a negative income (a cost). Store it as a
			// positive "paid" amount; it is subtracted where it enters PnL.
			l.Commissions += -usd
		}
	}
	return l
}

// incomeFetcher fetches one page of income rows for a symbol+type window. The
// real client implements it over go-binance; tests supply a mock so the
// aggregation/pagination loop runs with no network.
type incomeFetcher interface {
	fetchIncomePage(ctx context.Context, symbol, incomeType string, startMs, endMs, limit int64) ([]IncomeRow, error)
}

// collectIncome pulls every income row for the given symbols and the hedge
// income types over [startMs, endMs], paging on the endpoint's ~1000-row limit.
// Rows come back ascending by time; when a page is full we advance past the last
// row's timestamp and fetch again. (Rows sharing the exact last-row millisecond
// across a page boundary could be missed — acceptable for this version.)
func collectIncome(ctx context.Context, f incomeFetcher, symbols []string, startMs, endMs int64) ([]IncomeRow, error) {
	var all []IncomeRow
	for _, sym := range symbols {
		for _, it := range hedgeIncomeTypes {
			start := startMs
			for {
				page, err := f.fetchIncomePage(ctx, sym, it, start, endMs, incomePageLimit)
				if err != nil {
					return nil, err
				}
				all = append(all, page...)
				if int64(len(page)) < incomePageLimit {
					break
				}
				last := page[len(page)-1].Time
				if last < start {
					break // no forward progress; stop rather than loop forever
				}
				start = last + 1
			}
		}
	}
	return all, nil
}

// ledgerSince aggregates the income ledger from inception to now, clamping the
// start to the lookback window and flagging Partial when inception is older.
func ledgerSince(ctx context.Context, f incomeFetcher, symbols []string, inceptionMs, nowMs int64) (IncomeLedger, error) {
	partial := false
	start := inceptionMs
	earliest := nowMs - IncomeLookback.Milliseconds()
	if start < earliest {
		start = earliest
		partial = true
	}
	rows, err := collectIncome(ctx, f, symbols, start, nowMs)
	if err != nil {
		return IncomeLedger{}, err
	}
	led := AggregateIncome(rows)
	led.Partial = partial
	return led, nil
}

// GetHedgeIncome re-derives the cumulative hedge ledger (realized PnL, funding,
// commissions) for the given perp symbols from strategy inception to now. It is
// read-only and places no orders. When inception predates the lookback window
// the returned ledger has Partial set.
func (c *Client) GetHedgeIncome(ctx context.Context, symbols []string, inceptionMs int64) (IncomeLedger, error) {
	return ledgerSince(ctx, clientIncomeFetcher{c}, symbols, inceptionMs, time.Now().UnixMilli())
}

// clientIncomeFetcher adapts the live Binance client to incomeFetcher.
type clientIncomeFetcher struct{ c *Client }

func (f clientIncomeFetcher) fetchIncomePage(ctx context.Context, symbol, incomeType string, startMs, endMs, limit int64) ([]IncomeRow, error) {
	svc := f.c.futures.NewGetIncomeHistoryService().
		Symbol(symbol).
		IncomeType(incomeType).
		Limit(limit)
	if startMs > 0 {
		svc = svc.StartTime(startMs)
	}
	if endMs > 0 {
		svc = svc.EndTime(endMs)
	}
	rows, err := svc.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("income history %s %s: %w", symbol, incomeType, err)
	}
	out := make([]IncomeRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, IncomeRow{
			Symbol:     r.Symbol,
			Asset:      r.Asset,
			IncomeType: r.IncomeType,
			Income:     parseFloatOrZero(r.Income),
			Time:       r.Time,
		})
	}
	return out, nil
}
