// Package datasource fetches the raw pool and price data the analyzer needs.
//
// It defines a small interface so the scanner can be driven by either a live
// market data provider (GeckoTerminal for pools/OHLCV, Deribit for options
// implied volatility) or a self-contained synthetic provider for demos and
// offline development.
package datasource

import "context"

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
	Chain        string    `json:"chain"`        // display name
	ChainSlug    string    `json:"chainSlug"`    // provider slug
	ChainKind    ChainKind `json:"chainKind"`    // L1 / L2
	Address      string    `json:"address"`      // pool contract address
	DEX          string    `json:"dex"`          // exchange name
	Name         string    `json:"name"`         // e.g. "WETH / USDC"
	BaseSymbol   string    `json:"baseSymbol"`   // volatile asset symbol
	QuoteSymbol  string    `json:"quoteSymbol"`  // quote asset symbol
	FeeTier      float64   `json:"feeTier"`      // fractional fee (0.003 == 0.3%)
	TVLUSD       float64   `json:"tvlUsd"`       // total value locked
	Volume24hUSD float64   `json:"volume24hUsd"` // trailing 24h volume
	PriceUSD     float64   `json:"priceUsd"`     // current base price in USD

	// Closes holds historical base-asset close prices, oldest first, used to
	// measure realised volatility. PeriodsPerYear records their sampling rate.
	Closes         []float64 `json:"closes"`
	PeriodsPerYear float64   `json:"periodsPerYear"`
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
