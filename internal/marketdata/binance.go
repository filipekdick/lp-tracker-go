// Package marketdata provides unauthenticated market-data feeds for valuation.
package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBinanceFuturesURL = "https://fapi.binance.com"

// PriceStore is the narrow persistence contract required by the price cache.
type PriceStore interface {
	LatestPrice(ctx context.Context, symbol string, maxAge time.Duration) (float64, bool, error)
	SavePrice(ctx context.Context, symbol, source string, price float64, observedAt time.Time) error
}

// BinanceMarkPrices reads the public USD-M futures mark-price endpoint. It does
// not use credentials; only trading/account operations require Binance API keys.
type BinanceMarkPrices struct {
	baseURL string
	http    *http.Client
	store   PriceStore
	ttl     time.Duration
}

func NewBinanceMarkPrices(baseURL string, store PriceStore, ttl time.Duration) *BinanceMarkPrices {
	if baseURL == "" {
		baseURL = defaultBinanceFuturesURL
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &BinanceMarkPrices{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
		store:   store,
		ttl:     ttl,
	}
}

// Price returns a cached price when it is fresh; otherwise it requests the
// public Binance mark price and appends the observation to PostgreSQL.
func (b *BinanceMarkPrices) Price(ctx context.Context, symbol string) (float64, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return 0, fmt.Errorf("empty Binance symbol")
	}
	if b.store != nil {
		if price, ok, err := b.store.LatestPrice(ctx, symbol, b.ttl); err == nil && ok {
			return price, nil
		} else if err != nil {
			return 0, err
		}
	}

	u := b.baseURL + "/fapi/v1/premiumIndex?symbol=" + url.QueryEscape(symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	res, err := b.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Binance mark price %s: %w", symbol, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Binance mark price %s: status %d: %s", symbol, res.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		MarkPrice string `json:"markPrice"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("decode Binance mark price %s: %w", symbol, err)
	}
	price, err := strconv.ParseFloat(payload.MarkPrice, 64)
	if err != nil || price <= 0 {
		return 0, fmt.Errorf("invalid Binance mark price %q for %s", payload.MarkPrice, symbol)
	}
	if b.store != nil {
		if err := b.store.SavePrice(ctx, symbol, "binance_mark", price, time.Now()); err != nil {
			return 0, err
		}
	}
	return price, nil
}
