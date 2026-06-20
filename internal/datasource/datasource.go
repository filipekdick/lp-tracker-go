// Package datasource fetches the raw pool and price data the analyzer needs.
//
// It defines a small interface so the scanner can be driven by either a live
// market data provider (GeckoTerminal for pools/OHLCV, Deribit for options
// implied volatility) or a self-contained synthetic provider for demos and
// offline development.
package datasource

import (
	"context"
	"strings"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
)

// ChainKind distinguishes settlement layers.
type ChainKind string

const (
	L1 ChainKind = "L1"
	L2 ChainKind = "L2"
)

// Chain describes a network to scan. Slug is the provider's network identifier
// (e.g. GeckoTerminal uses "eth", "arbitrum", "base").
type Chain struct {
	Slug    string    `json:"slug"`
	Display string    `json:"display"`
	Kind    ChainKind `json:"kind"`
}

// DefaultChains is a representative spread of L1s and L2s.
var DefaultChains = []Chain{
	{Slug: "eth", Display: "Ethereum", Kind: L1},
	{Slug: "bsc", Display: "BNB Chain", Kind: L1},
	{Slug: "solana", Display: "Solana", Kind: L1},
	{Slug: "avax", Display: "Avalanche", Kind: L1},
	{Slug: "arbitrum", Display: "Arbitrum", Kind: L2},
	{Slug: "base", Display: "Base", Kind: L2},
	{Slug: "optimism", Display: "Optimism", Kind: L2},
	{Slug: "polygon_pos", Display: "Polygon", Kind: L2},
}

// RawPool is one pool's market data as returned by a Source, before analysis.
type RawPool struct {
	Chain         string    `json:"chain"`         // display name
	ChainSlug     string    `json:"chainSlug"`     // provider slug
	ChainKind     ChainKind `json:"chainKind"`     // L1 / L2
	Address       string    `json:"address"`       // pool contract address
	DEX           string    `json:"dex"`           // raw exchange id (e.g. "aerodrome_slipstream")
	Protocol      string    `json:"protocol"`      // normalised family (e.g. "Aerodrome")
	Name          string    `json:"name"`          // e.g. "WETH / USDC"
	BaseSymbol    string    `json:"baseSymbol"`    // volatile asset symbol
	QuoteSymbol   string    `json:"quoteSymbol"`   // quote asset symbol
	FeeTier       float64   `json:"feeTier"`       // fractional fee (0.003 == 0.3%)
	TVLUSD        float64   `json:"tvlUsd"`        // total value locked
	Volume24hUSD  float64   `json:"volume24hUsd"`  // trailing 24h volume
	PriceUSD      float64   `json:"priceUsd"`      // current base price in USD
	QuotePriceUSD float64   `json:"quotePriceUsd"` // current quote price in USD

	// Closes holds historical base-asset close prices, oldest first, used to
	// measure realised volatility. PeriodsPerYear records their sampling rate.
	// Closes is the 7-day (168-bar) tail, preserved so the default analysis is
	// unchanged and the dashboard sparkline stays 7d.
	Closes         []float64 `json:"closes"`
	PeriodsPerYear float64   `json:"periodsPerYear"`

	// OHLC holds the full OHLCV history (oldest first), long enough for the
	// longest volatility window (14 days). It is fetched in the SAME single
	// OHLCV request that produces Closes, and feeds the selectable volatility
	// methods. Not serialised to clients (they receive precomputed metrics) to
	// keep the API payload small.
	OHLC []analyzer.OHLCV `json:"-"`
}

// ProtocolFamily normalises a provider DEX id into a human-readable protocol
// family, so e.g. "aerodrome_slipstream" -> "Aerodrome" and "uniswap_v3" ->
// "Uniswap". Unknown ids are title-cased best-effort.
func ProtocolFamily(dex string) string {
	d := strings.ToLower(dex)
	switch {
	case strings.Contains(d, "aerodrome"):
		return "Aerodrome"
	case strings.Contains(d, "velodrome"):
		return "Velodrome"
	case strings.Contains(d, "pancake"):
		return "PancakeSwap"
	case strings.Contains(d, "uniswap"):
		return "Uniswap"
	case strings.Contains(d, "sushi"):
		return "SushiSwap"
	case strings.Contains(d, "curve"):
		return "Curve"
	case strings.Contains(d, "camelot"):
		return "Camelot"
	case strings.Contains(d, "quickswap"):
		return "QuickSwap"
	case strings.Contains(d, "thena"):
		return "Thena"
	case strings.Contains(d, "trader_joe") || strings.Contains(d, "traderjoe"):
		return "Trader Joe"
	case strings.Contains(d, "pharaoh"):
		return "Pharaoh"
	case strings.Contains(d, "orca"):
		return "Orca"
	case strings.Contains(d, "raydium"):
		return "Raydium"
	case strings.Contains(d, "meteora"):
		return "Meteora"
	}
	if d == "" {
		return "Unknown"
	}
	// Fallback: drop a trailing version suffix and title-case.
	d = strings.TrimRight(d, "0123456789_")
	d = strings.ReplaceAll(d, "_", " ")
	parts := strings.Fields(d)
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// Source returns the most active pools for a set of chains.
type Source interface {
	Name() string
	TopPools(ctx context.Context, chains []Chain, perChain int) ([]RawPool, error)
}

// ImpliedVolSource supplies options-implied annualised volatility for an asset
// symbol. ok is false when no implied vol is available for that symbol.
type ImpliedVolSource interface {
	ImpliedVol(ctx context.Context, symbol string) (vol float64, ok bool)
}
