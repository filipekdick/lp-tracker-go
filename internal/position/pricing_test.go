package position

import (
	"context"
	"errors"
	"math"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/lp"
	"github.com/filipekdick/lp-tracker-go/internal/marketdata"
)

type testQuotes struct {
	calls int
	fail  bool
}

func (q *testQuotes) Quote(context.Context, string) (marketdata.Quote, error) {
	q.calls++
	if q.fail {
		return marketdata.Quote{}, errors.New("offline")
	}
	return marketdata.Quote{Price: 2000, ObservedAt: time.Now()}, nil
}

type testPools struct {
	calls atomic.Int32
	gate  chan struct{}
	fail  bool
}

func (p *testPools) PoolByAddress(ctx context.Context, c datasource.Chain, a string) (datasource.RawPool, error) {
	p.calls.Add(1)
	if p.gate != nil {
		select {
		case <-p.gate:
		case <-ctx.Done():
			return datasource.RawPool{}, ctx.Err()
		}
	}
	if p.fail {
		return datasource.RawPool{}, errors.New("Gecko unavailable")
	}
	return datasource.RawPool{Address: a, ChainSlug: c.Slug, TVLUSD: 1000}, nil
}
func reportRatio(ratio float64, d0, d1 uint8) lp.PositionReport {
	raw := math.Sqrt(ratio / math.Pow10(int(d0)-int(d1)))
	f := new(big.Float).SetPrec(256).Mul(new(big.Float).SetFloat64(raw), new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 96)))
	sqrt, _ := f.Int(nil)
	return lp.PositionReport{TokenID: 1, Symbol0: "WETH", Symbol1: "USDC", Decimals0: d0, Decimals1: d1, SqrtPriceX96: sqrt, Amount0: 2, Amount1: 100, ObservedAt: time.Now()}
}
func closePrice(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > want*1e-10 {
		t.Fatalf("got %g want %g", got, want)
	}
}

func TestOnchainStablePricingWithoutGeckoOrBinance(t *testing.T) {
	q := &testQuotes{fail: true}
	g := &testPools{fail: true}
	tracker := &LiveTracker{prices: q, pricer: g}
	r := reportRatio(2000, 18, 6)
	leg := tracker.priceLeg(context.Background(), r).leg
	closePrice(t, leg.Price0, 2000)
	closePrice(t, leg.ValueUSD, 4100)
	if !leg.ValuationAvailable || leg.Error != "" || q.calls != 0 || leg.AnalysisStatus == "fresh" {
		t.Fatalf("unexpected valuation: %+v calls=%d", leg, q.calls)
	}
	if leg.Price1Info.Assumption == "" || leg.Price0Info.Source != "onchain_sqrt_price" {
		t.Fatal("missing provenance")
	}
	r = reportRatio(1.0/2000, 6, 18)
	r.Symbol0, r.Symbol1 = "USDC", "WETH"
	a, b, _, _ := tracker.valueOnChain(context.Background(), r)
	closePrice(t, a, 1)
	closePrice(t, b, 2000)
}
func TestVolatilePairAnchorAndFailure(t *testing.T) {
	q := &testQuotes{}
	tracker := &LiveTracker{prices: q}
	r := reportRatio(2, 18, 18)
	r.Symbol0, r.Symbol1 = "wstETH", "WETH"
	a, b, ma, mb := tracker.valueOnChain(context.Background(), r)
	closePrice(t, a, 4000)
	closePrice(t, b, 2000)
	if q.calls != 1 || ma.AnchorObservedAt.IsZero() || mb.Source != "binance_mark_usdt" {
		t.Fatal("wrong anchor metadata")
	}
	q.fail = true
	leg := tracker.priceLeg(context.Background(), r).leg
	if leg.ValuationAvailable || leg.Error == "" {
		t.Fatal("missing anchor must be unavailable")
	}
	r.SqrtPriceX96 = nil
	a, b, _, _ = tracker.valueOnChain(context.Background(), r)
	if a != 0 || b != 0 {
		t.Fatal("missing ratio priced")
	}
}
func TestAnalysisRefreshNonblockingAndDeduplicated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	upstream := &testPools{gate: make(chan struct{})}
	c := NewCachedPoolPricer(upstream, nil, time.Hour).(*cachedPoolPricer)
	chain := datasource.Chain{Slug: "base"}
	// Calls return while upstream remains blocked, including repeated requests.
	for i := 0; i < 10; i++ {
		if _, err := c.PoolByAddress(ctx, chain, "pool"); err == nil {
			t.Fatal("cold cache reported fresh")
		}
	}
	close(upstream.gate)
	deadline := time.Now().Add(time.Second)
	for {
		p, err := c.PoolByAddress(ctx, chain, "pool")
		if err == nil && !p.ObservedAt.IsZero() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	if upstream.calls.Load() != 1 {
		t.Fatal("duplicate upstream calls")
	}
	// Stale analysis remains usable and failure is backed off.
	c.mu.Lock()
	e := c.entries["base:pool"]
	e.pool.ObservedAt = time.Now().Add(-2 * time.Hour)
	c.entries["base:pool"] = e
	c.mu.Unlock()
	upstream.fail = true
	p, err := c.PoolByAddress(ctx, chain, "pool")
	if err == nil || p.TVLUSD != 1000 {
		t.Fatal("stale snapshot not retained")
	}
	deadline = time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		done := !c.entries["base:pool"].loading
		c.mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed refresh hung")
		}
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		c.PoolByAddress(ctx, chain, "pool")
	}
	if upstream.calls.Load() != 2 {
		t.Fatal("failure retry storm")
	}
}

type testReader struct{ report lp.PositionReport }

func (r *testReader) ReadPosition(int64) (lp.PositionReport, error) { return r.report, nil }
func TestMissingPriceDoesNotAppendFalseLoss(t *testing.T) {
	r := &testReader{report: reportRatio(2000, 18, 6)}
	tracker := NewLiveTracker(r, nil, nil, nil, nil, []int64{1}, datasource.Chain{}, true, nil, nil)
	first, err := tracker.Track(context.Background())
	if err != nil || len(first.History) != 1 {
		t.Fatalf("first snapshot: %v %+v", err, first)
	}
	r.report.Symbol1 = "UNKNOWN"
	second, err := tracker.Track(context.Background())
	if err != nil || second.ValuationAvailable || len(second.History) != 1 {
		t.Fatalf("missing price appended snapshot: %v %+v", err, second)
	}
	r.report.Symbol1 = "USDC"
	third, err := tracker.Track(context.Background())
	if err != nil || len(third.History) != 2 || third.History[1].NetPnL != 0 {
		t.Fatalf("recovery: %v %+v", err, third)
	}
}

func TestEmptyHedgeRetainsDryRun(t *testing.T) {
	tracker := &LiveTracker{dryRun: true}
	h := tracker.hedges(context.Background(), nil)
	if len(h) != 1 || !h[0].DryRun || h[0].Available {
		t.Fatal("empty hedge lost dry-run state")
	}
	if tracker.CancelOrder(context.Background(), "ETHUSDT", 1) == nil {
		t.Fatal("dry-run permitted cancellation")
	}
}
