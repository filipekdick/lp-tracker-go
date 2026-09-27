package position

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/lp"
	"github.com/filipekdick/lp-tracker-go/internal/marketdata"
	"github.com/filipekdick/lp-tracker-go/internal/v3math"
)

// PriceInfo records both the pool observation and the USD anchor's timestamp.
// Stablecoin parity is an explicit assumption, never an observed dollar quote.
type PriceInfo struct {
	Source           string    `json:"source"`
	ObservedAt       time.Time `json:"observedAt"`
	AnchorSource     string    `json:"anchorSource"`
	AnchorObservedAt time.Time `json:"anchorObservedAt"`
	Cached           bool      `json:"cached"`
	Stale            bool      `json:"stale"`
	Assumption       string    `json:"assumption,omitempty"`
}

func validPrice(p float64) bool { return p > 0 && !math.IsNaN(p) && !math.IsInf(p, 0) }

// Only direct assets and 1:1 wrappers can use a Binance reference. Yield-bearing
// wrappers (wstETH/cbETH etc.) must obtain their exchange ratio from the pool.
func anchorPerp(symbol string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	switch s {
	case "WETH":
		s = "ETH"
	case "WBTC", "CBBTC":
		s = "BTC"
	case "WBNB":
		s = "BNB"
	}
	if _, ok := hedgeBaseAssets[s]; !ok {
		return "", false
	}
	return s + "USDT", true
}

func (t *LiveTracker) valueOnChain(ctx context.Context, r lp.PositionReport) (float64, float64, PriceInfo, PriceInfo) {
	ratio := v3math.PriceAtSqrtRatio(r.SqrtPriceX96, r.Decimals0, r.Decimals1)
	if !validPrice(ratio) {
		return 0, 0, PriceInfo{}, PriceInfo{}
	}
	symbols := []string{r.Symbol0, r.Symbol1}
	// Prefer an explicit stablecoin anchor, avoiding any price API request.
	for i, s := range symbols {
		if stableSymbols[strings.ToUpper(strings.TrimSpace(s))] {
			meta := PriceInfo{Source: "stablecoin_assumption", Assumption: s + " = USD 1 (assumed parity)"}
			derived := meta
			derived.Source = "onchain_sqrt_price"
			derived.ObservedAt = r.ObservedAt
			derived.AnchorSource = meta.Source
			if i == 1 {
				return ratio, 1, derived, meta
			}
			return 1, 1 / ratio, meta, derived
		}
	}
	if t.prices != nil {
		for i, s := range symbols {
			perp, ok := anchorPerp(s)
			if !ok {
				continue
			}
			q, err := t.prices.Quote(ctx, perp)
			if err != nil || !validPrice(q.Price) {
				continue
			}
			meta := infoFromQuote(q)
			derived := meta
			derived.Source = "onchain_sqrt_price"
			derived.ObservedAt = r.ObservedAt
			derived.AnchorSource = meta.Source
			derived.AnchorObservedAt = q.ObservedAt
			if i == 0 {
				return q.Price, q.Price / ratio, meta, derived
			}
			return q.Price * ratio, q.Price, derived, meta
		}
	}
	return 0, 0, PriceInfo{}, PriceInfo{}
}

func infoFromQuote(q marketdata.Quote) PriceInfo {
	return PriceInfo{Source: "binance_mark_usdt", ObservedAt: q.ObservedAt, Cached: q.Cached, Stale: q.Stale, Assumption: "USDT = USD 1 (quote currency)"}
}
