package position

// aggregate.go folds several LP positions into a single, simplified hedge.
//
// A wallet often holds the SAME underlying exposure spread across many LP
// positions — e.g. ETH in a WETH/USDC pool, more ETH in a wstETH/WETH pool, and
// yet more in a cbETH/USDC pool. Hedging each leg with its own short is wasteful
// and noisy. Instead we sum the volatile-leg units of every position by their
// NORMALIZED asset (so WETH, wstETH, cbETH and stETH all roll into ETH — the
// "synthetics" of the same asset) and place ONE short per asset sized to the
// total. The result is a single, simplified hedge book that tracks the net
// directional exposure of the whole LP portfolio.

import (
	"sort"
	"strings"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
)

// PositionLeg is one tracked LP position's state, as a member of an aggregated
// portfolio. It mirrors the per-pool fields of TrackedPosition so the dashboard
// can render each underlying position individually while the hedge is reported
// once for the whole portfolio.
type PositionLeg struct {
	TokenID     int64                `json:"tokenId"`
	Chain       string               `json:"chain"`
	ChainSlug   string               `json:"chainSlug"`
	ChainKind   datasource.ChainKind `json:"chainKind"`
	Protocol    string               `json:"protocol"`
	DEX         string               `json:"dex"`
	PoolAddress string               `json:"poolAddress"`
	PoolName    string               `json:"poolName"`
	Symbol0     string               `json:"symbol0"`
	Symbol1     string               `json:"symbol1"`
	Amount0     float64              `json:"amount0"`
	Amount1     float64              `json:"amount1"`
	TickLower   int64                `json:"tickLower"`
	TickUpper   int64                `json:"tickUpper"`
	TickNow     int64                `json:"tickNow"`
	InRange     bool                 `json:"inRange"`
	Decimals0   uint8                `json:"decimals0"`
	Decimals1   uint8                `json:"decimals1"`

	RangeLowerPrice   float64 `json:"rangeLowerPrice"`
	RangeUpperPrice   float64 `json:"rangeUpperPrice"`
	RangeCurrentPrice float64 `json:"rangeCurrentPrice"`
	RangePositionPct  float64 `json:"rangePositionPct"`

	UncollectedFees0 float64 `json:"uncollectedFees0"`
	UncollectedFees1 float64 `json:"uncollectedFees1"`

	ValueUSD     float64 `json:"valueUsd"`
	Price0       float64 `json:"price0"`
	Price1       float64 `json:"price1"`
	FeeTier      float64 `json:"feeTier"`
	TVLUSD       float64 `json:"tvlUsd"`
	Volume24hUSD float64 `json:"volume24hUsd"`

	Analysis analyzer.Result `json:"analysis"`
	Error    string          `json:"error,omitempty"`
}

// ExposureSource attributes a slice of an aggregated exposure back to the LP
// position (and the exact pool token, e.g. "wstETH") it came from.
type ExposureSource struct {
	TokenID int64   `json:"tokenId"`
	Symbol  string  `json:"symbol"` // the original pool token symbol
	Amount  float64 `json:"amount"` // units of that token in this position
}

// AssetExposure is the summed volatile-leg exposure of ONE underlying asset
// across every tracked LP position, with the synthetics folded in. It is hedged
// by a single perp short of Amount units on Perp.
type AssetExposure struct {
	Asset         string           `json:"asset"`         // normalized asset, e.g. "ETH"
	DisplaySymbol string           `json:"displaySymbol"` // representative pool symbol, e.g. "WETH"
	Perp          string           `json:"perp"`          // resolved perp symbol, e.g. "ETHUSDC"
	Amount        float64          `json:"amount"`        // total units across all positions
	Sources       []ExposureSource `json:"sources"`       // per-position breakdown
}

// aggregateExposures sums the hedgeable volatile-leg units of every position by
// normalized asset, folding synthetics together (WETH + wstETH -> ETH). It
// returns one AssetExposure per asset, ordered by descending total units so the
// largest exposure leads. Stable legs and assets without a listed perp are
// skipped.
func aggregateExposures(legs []PositionLeg) []AssetExposure {
	byAsset := map[string]*AssetExposure{}

	add := func(tokenID int64, sym string, amt float64) {
		if amt <= 0 {
			return
		}
		if stableSymbols[strings.ToUpper(strings.TrimSpace(sym))] {
			return
		}
		perp, ok := perpForSymbol(sym)
		if !ok {
			return
		}
		norm := datasource.NormalizeSymbol(sym)
		ex, has := byAsset[norm]
		if !has {
			ex = &AssetExposure{Asset: norm, DisplaySymbol: sym, Perp: perp}
			byAsset[norm] = ex
		}
		ex.Amount += amt
		ex.Sources = append(ex.Sources, ExposureSource{TokenID: tokenID, Symbol: sym, Amount: amt})
	}

	for _, leg := range legs {
		add(leg.TokenID, leg.Symbol0, leg.Amount0)
		add(leg.TokenID, leg.Symbol1, leg.Amount1)
	}

	out := make([]AssetExposure, 0, len(byAsset))
	for _, ex := range byAsset {
		out = append(out, *ex)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].Asset < out[j].Asset
	})
	return out
}
