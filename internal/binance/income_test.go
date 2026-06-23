package binance

import (
	"context"
	"math"
	"testing"
	"time"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestAggregateIncomeSumsAndSigns checks the three running sums and their signs:
// realized PnL accumulates across multiple closed legs, funding nets a positive
// and a negative row with the correct sign, and commissions accumulate as a
// positive paid cost from Binance's negative income rows.
func TestAggregateIncomeSumsAndSigns(t *testing.T) {
	rows := []IncomeRow{
		{IncomeType: "REALIZED_PNL", Asset: "USDT", Income: 12.5}, // closed leg 1: gain
		{IncomeType: "REALIZED_PNL", Asset: "USDT", Income: -4.0}, // closed leg 2: loss
		{IncomeType: "REALIZED_PNL", Asset: "USDC", Income: 1.5},  // closed leg 3: gain
		{IncomeType: "FUNDING_FEE", Asset: "USDT", Income: 3.0},   // short received funding
		{IncomeType: "FUNDING_FEE", Asset: "USDT", Income: -1.25}, // short paid funding
		{IncomeType: "COMMISSION", Asset: "USDT", Income: -0.8},   // fee paid
		{IncomeType: "COMMISSION", Asset: "USDC", Income: -0.2},   // fee paid
	}
	l := AggregateIncome(rows)
	if !approx(l.RealizedPnL, 10.0) { // 12.5 - 4.0 + 1.5
		t.Errorf("realized = %v, want 10.0", l.RealizedPnL)
	}
	if !approx(l.Funding, 1.75) { // 3.0 - 1.25
		t.Errorf("funding = %v, want 1.75", l.Funding)
	}
	if !approx(l.Commissions, 1.0) { // -(-0.8) + -(-0.2) = 1.0 paid
		t.Errorf("commissions = %v, want 1.0", l.Commissions)
	}
	if l.Partial {
		t.Error("AggregateIncome should not set Partial")
	}
}

// TestAggregateIncomeStablecoinParity checks USDT and USDC are both ~1 USD, so
// the same nominal income in either margin asset contributes equally.
func TestAggregateIncomeStablecoinParity(t *testing.T) {
	usdt := AggregateIncome([]IncomeRow{{IncomeType: "REALIZED_PNL", Asset: "USDT", Income: 7.0}})
	usdc := AggregateIncome([]IncomeRow{{IncomeType: "REALIZED_PNL", Asset: "USDC", Income: 7.0}})
	if !approx(usdt.RealizedPnL, usdc.RealizedPnL) || !approx(usdt.RealizedPnL, 7.0) {
		t.Errorf("USDT %v vs USDC %v, want both 7.0", usdt.RealizedPnL, usdc.RealizedPnL)
	}
}

// mockFetcher serves income pages from a fixed table, recording the windows it
// was asked for. It splits rows into ~incomePageLimit-sized pages to exercise
// pagination, and only returns rows whose time falls in the requested window so
// the lookback clamp is observable.
type mockFetcher struct {
	rows     map[string][]IncomeRow // keyed by symbol|incomeType
	minStart int64                  // smallest startMs the fetcher was asked for
	calls    int
}

func (m *mockFetcher) fetchIncomePage(_ context.Context, symbol, incomeType string, startMs, endMs, limit int64) ([]IncomeRow, error) {
	m.calls++
	if m.minStart == 0 || startMs < m.minStart {
		m.minStart = startMs
	}
	var in []IncomeRow
	for _, r := range m.rows[symbol+"|"+incomeType] {
		if r.Time >= startMs && (endMs == 0 || r.Time <= endMs) {
			in = append(in, r)
		}
	}
	if int64(len(in)) > limit {
		return in[:limit], nil
	}
	return in, nil
}

// TestPartialWindowFlag verifies the ledger is flagged Partial when inception
// predates the income lookback window, and not when it falls inside it.
func TestPartialWindowFlag(t *testing.T) {
	now := time.Now().UnixMilli()
	syms := []string{"ETHUSDT"}
	f := &mockFetcher{rows: map[string][]IncomeRow{
		"ETHUSDT|REALIZED_PNL": {{IncomeType: "REALIZED_PNL", Asset: "USDT", Income: 5, Time: now - 1000}},
	}}

	// Inception well inside the window → not partial.
	inside := now - (10 * 24 * time.Hour).Milliseconds()
	l, err := ledgerSince(context.Background(), f, syms, inside, now)
	if err != nil {
		t.Fatal(err)
	}
	if l.Partial {
		t.Error("inception inside lookback should not be Partial")
	}

	// Inception older than the window → partial, and the fetch is clamped to the
	// lookback start rather than the (older) inception.
	old := now - (200 * 24 * time.Hour).Milliseconds()
	f2 := &mockFetcher{rows: f.rows}
	l2, err := ledgerSince(context.Background(), f2, syms, old, now)
	if err != nil {
		t.Fatal(err)
	}
	if !l2.Partial {
		t.Error("inception older than lookback should be Partial")
	}
	earliest := now - IncomeLookback.Milliseconds()
	if f2.minStart < earliest {
		t.Errorf("fetch start %d should be clamped to lookback start %d", f2.minStart, earliest)
	}
}

// TestPaginationAggregatesAllPages checks the paging loop fetches every page
// when the first comes back full (== incomePageLimit) and aggregates them all.
func TestPaginationAggregatesAllPages(t *testing.T) {
	now := time.Now().UnixMilli()
	// 1500 realized rows of +1 each, ascending in time, so the first page is full
	// (1000) and a second page (500) follows.
	var realized []IncomeRow
	for i := 0; i < 1500; i++ {
		realized = append(realized, IncomeRow{
			IncomeType: "REALIZED_PNL", Asset: "USDT", Income: 1, Time: now - int64(1500-i),
		})
	}
	f := &mockFetcher{rows: map[string][]IncomeRow{"ETHUSDT|REALIZED_PNL": realized}}
	l, err := ledgerSince(context.Background(), f, []string{"ETHUSDT"}, now-100000, now)
	if err != nil {
		t.Fatal(err)
	}
	if !approx(l.RealizedPnL, 1500) {
		t.Errorf("realized = %v, want 1500 (all pages aggregated)", l.RealizedPnL)
	}
}
