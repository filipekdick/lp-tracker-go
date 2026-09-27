// Package marketdata provides unauthenticated market-data feeds for valuation.
package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/metrics"
)

const defaultBinanceFuturesURL = "https://fapi.binance.com"

type Quote struct {
	Price      float64
	ObservedAt time.Time
	Cached     bool
	Stale      bool
}

type PriceStore interface {
	LatestPrice(context.Context, string, time.Duration) (float64, time.Time, bool, error)
	SavePrice(context.Context, string, string, float64, time.Time) error
}

type BinanceMarkPrices struct {
	baseURL string
	http    *http.Client
	store   PriceStore
	ttl     time.Duration
	mu      sync.Mutex
	quotes  map[string]Quote
}

func NewBinanceMarkPrices(baseURL string, store PriceStore, ttl time.Duration) *BinanceMarkPrices {
	if baseURL == "" {
		baseURL = defaultBinanceFuturesURL
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &BinanceMarkPrices{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 10 * time.Second}, store: store, ttl: ttl, quotes: map[string]Quote{}}
}

func (b *BinanceMarkPrices) Price(ctx context.Context, symbol string) (float64, error) {
	q, err := b.Quote(ctx, symbol)
	return q.Price, err
}

// Quote preserves the original observation time across memory/database cache
// hits. Expired prices are never silently reused; DB failure leaves the public
// feed available. A mutex coalesces concurrent requests for the same quote.
func (b *BinanceMarkPrices) Quote(ctx context.Context, symbol string) (Quote, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return Quote{}, fmt.Errorf("empty Binance symbol")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if q, ok := b.quotes[symbol]; ok && time.Since(q.ObservedAt) < b.ttl {
		q.Cached = true
		metrics.Inc("binance_price_cache_hits")
		return q, nil
	}
	if b.store != nil {
		if p, at, ok, err := b.store.LatestPrice(ctx, symbol, b.ttl); err == nil && ok && valid(p) && !at.IsZero() && time.Since(at) < b.ttl {
			q := Quote{Price: p, ObservedAt: at, Cached: true}
			b.quotes[symbol] = q
			metrics.Inc("binance_price_cache_hits")
			return q, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+"/fapi/v1/premiumIndex?symbol="+url.QueryEscape(symbol), nil)
	if err != nil {
		return Quote{}, err
	}
	metrics.Inc("binance_price_requests")
	res, err := b.http.Do(req)
	if err != nil {
		metrics.Inc("binance_price_errors")
		return Quote{}, fmt.Errorf("Binance mark price %s: %w", symbol, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Quote{}, err
	}
	if res.StatusCode != http.StatusOK {
		metrics.Inc("binance_price_errors")
		return Quote{}, fmt.Errorf("Binance mark price %s: status %d", symbol, res.StatusCode)
	}
	var payload struct {
		MarkPrice string `json:"markPrice"`
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		return Quote{}, err
	}
	p, err := strconv.ParseFloat(payload.MarkPrice, 64)
	if err != nil || !valid(p) {
		return Quote{}, fmt.Errorf("invalid Binance mark price")
	}
	q := Quote{Price: p, ObservedAt: time.Now()}
	b.quotes[symbol] = q
	if b.store != nil {
		_ = b.store.SavePrice(ctx, symbol, "binance_mark", p, q.ObservedAt)
	}
	return q, nil
}
func valid(p float64) bool { return p > 0 && !math.IsNaN(p) && !math.IsInf(p, 0) }
