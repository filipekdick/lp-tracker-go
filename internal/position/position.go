// Package position tracks a single liquidity-pool position, runs the
// fee-versus-volatility model over its pool, and reports the perpetual-futures
// short required to hedge the position's directional exposure.
//
// It defines a Tracker interface with two implementations: a live tracker that
// reads the position from chain (via internal/lp), prices the pool from
// GeckoTerminal, pulls options-implied volatility from Deribit and reads the
// current hedge from Binance futures; and a demo tracker that synthesises a
// realistic position so the dashboard works with no external connectivity.
package position

import (
	"context"
	"strings"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/v3math"
)

// fillRangePrices populates the notional-price range fields from the position's
// ticks and token decimals via internal/v3math. Display-only, no network/chain
// access. Safe to call once the ticks and decimals are set.
func (tp *TrackedPosition) fillRangePrices() {
	info := v3math.RangePrices(tp.TickLower, tp.TickUpper, tp.TickNow, tp.Decimals0, tp.Decimals1)
	tp.RangeLowerPrice = info.LowerPrice
	tp.RangeUpperPrice = info.UpperPrice
	tp.RangeCurrentPrice = info.CurrentPrice
	tp.RangePositionPct = info.WithinPct
}

// Hedge describes the perp short that offsets a position's volatile-leg exposure.
type Hedge struct {
	Venue          string  `json:"venue"`          // e.g. "Binance Futures (testnet)"
	Symbol         string  `json:"symbol"`         // perp symbol, e.g. "ETHUSDT"
	ExposureSymbol string  `json:"exposureSymbol"` // hedged asset, e.g. "WETH"
	LPExposure     float64 `json:"lpExposure"`     // volatile-leg units held in the LP
	TargetShort    float64 `json:"targetShort"`    // desired short size (base units)
	CurrentShort   float64 `json:"currentShort"`   // current short size (positive number)
	Drift          float64 `json:"drift"`          // TargetShort - CurrentShort
	EntryPrice     float64 `json:"entryPrice"`     // short entry price (0 if flat/unknown)
	MarkPrice      float64 `json:"markPrice"`      // current mark price
	UnrealizedPnL  float64 `json:"unrealizedPnl"`  // short PnL in USD
	NotionalUSD    float64 `json:"notionalUsd"`    // |currentShort| * markPrice
	InSync         bool    `json:"inSync"`         // |drift| within tolerance
	DryRun         bool    `json:"dryRun"`         // advisory only, no orders sent
	Available      bool    `json:"available"`      // false when no venue creds configured
	Note           string  `json:"note"`           // human-readable status
}

// TrackedPosition is the full picture of one LP position for the dashboard.
type TrackedPosition struct {
	// Identity / on-chain state.
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

	// Range as notional prices, derived from the ticks + decimals via
	// internal/v3math (display-only, no API/chain calls). RangePositionPct reads
	// 0% at the lower bound and 100% at the upper bound.
	RangeLowerPrice   float64 `json:"rangeLowerPrice"`
	RangeUpperPrice   float64 `json:"rangeUpperPrice"`
	RangeCurrentPrice float64 `json:"rangeCurrentPrice"`
	RangePositionPct  float64 `json:"rangePositionPct"`

	// Uncollected (live claimable) fees. TokensOwed0/1 are kept as a back-compat
	// alias of the uncollected amounts.
	UncollectedFees0 float64 `json:"uncollectedFees0"`
	UncollectedFees1 float64 `json:"uncollectedFees1"`
	TokensOwed0      float64 `json:"tokensOwed0"`
	TokensOwed1      float64 `json:"tokensOwed1"`

	// Valuation and pool market data.
	ValueUSD     float64 `json:"valueUsd"`
	Price0       float64 `json:"price0"` // current USD price of token0
	Price1       float64 `json:"price1"` // current USD price of token1
	FeeTier      float64 `json:"feeTier"`
	TVLUSD       float64 `json:"tvlUsd"`
	Volume24hUSD float64 `json:"volume24hUsd"`

	// Fee-versus-volatility analysis for this pool.
	Analysis analyzer.Result `json:"analysis"`

	// Hedge state.
	Hedges          []Hedge              `json:"hedges"`
	Hedge           Hedge                `json:"hedge"` // Backward compatibility
	OpenShorts      []binance.Position   `json:"openShorts"`
	OpenLimitOrders []binance.LimitOrder `json:"openLimitOrders"`

	// Hedge PnL ledger, cumulative USD since strategy inception, re-derived from
	// Binance income history each poll. Signs: realized and funding are signed
	// (funding positive = the short received funding); commissions is a positive
	// cost that is subtracted in net PnL. HedgeIncomePartial is true when
	// inception predates the income lookback window, so these totals are partial.
	HedgeRealizedPnL    float64 `json:"hedgeRealizedPnl"`
	HedgeFundingUSD     float64 `json:"hedgeFundingUsd"`
	HedgeCommissionsUSD float64 `json:"hedgeCommissionsUsd"`
	HedgeIncomePartial  bool    `json:"hedgeIncomePartial"`

	// LP fee ledger. FeesToCollectUSD is the current uncollected fees (what a
	// harvest realizes right now); FeesTotalUSD is the cumulative collected +
	// uncollected since inception, which survives harvests and never resets.
	FeesToCollectUSD float64 `json:"feesToCollectUsd"`
	FeesTotalUSD     float64 `json:"feesTotalUsd"`

	// History
	InitialState *Snapshot  `json:"initialState"`
	History      []Snapshot `json:"history"`

	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updatedAt"`
	Error     string    `json:"error,omitempty"`
}

// Snapshot stores a historical data point for graphing and PnL tracking.
type Snapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Price0    float64   `json:"price0"`
	Price1    float64   `json:"price1"`
	Amount0   float64   `json:"amount0"`
	Amount1   float64   `json:"amount1"`
	ValueUSD  float64   `json:"valueUsd"`
	// HedgePnL is the open short's unrealized PnL (the live, resettable part).
	HedgePnL float64 `json:"hedgePnl"`
	// HedgeRealizedPnL, HedgeFundingUSD and HedgeCommissionsUSD are cumulative USD
	// since inception (see TrackedPosition for signs).
	HedgeRealizedPnL    float64 `json:"hedgeRealizedPnl"`
	HedgeFundingUSD     float64 `json:"hedgeFundingUsd"`
	HedgeCommissionsUSD float64 `json:"hedgeCommissionsUsd"`
	// FeesUSD is the cumulative LP fees (collected + uncollected) in USD — the
	// value used by the net PnL identity and the chart, so it survives harvests.
	FeesUSD float64 `json:"feesUsd"`
	NetPnL  float64 `json:"netPnl"`
}

// Tracker produces the current state of a tracked position.
type Tracker interface {
	Name() string
	Track(ctx context.Context) (TrackedPosition, error)
	CancelOrder(ctx context.Context, symbol string, orderID int64) error
}

// hedgeFutures maps a (possibly wrapped) spot symbol to the perp used to hedge
// it. Only assets with a liquid linear perp are hedgeable.
var hedgeFutures = map[string]string{
	"ETH": "ETHUSDT", "BTC": "BTCUSDT", "SOL": "SOLUSDT",
	"ARB": "ARBUSDT", "OP": "OPUSDT", "AVAX": "AVAXUSDT",
	"LINK": "LINKUSDT", "MATIC": "MATICUSDT", "BNB": "BNBUSDT",
	"VVV": "VVVUSDT", "VIRTUAL": "VIRTUALUSDT",
}

// stableSymbols are treated as non-volatile (no hedge needed for that leg).
var stableSymbols = map[string]bool{
	"USDC": true, "USDT": true, "DAI": true, "USDC.E": true,
	"USDBC": true, "FRAX": true, "LUSD": true, "USDE": true,
}

type HedgeLeg struct {
	Asset  string
	Amount float64
	Perp   string
}

func hedgeLegs(sym0, sym1 string, amt0, amt1 float64) []HedgeLeg {
	var legs []HedgeLeg
	type candidate struct {
		sym string
		amt float64
	}
	for _, c := range []candidate{{sym0, amt0}, {sym1, amt1}} {
		if stableSymbols[strings.ToUpper(strings.TrimSpace(c.sym))] {
			continue
		}
		norm := datasource.NormalizeSymbol(c.sym)
		if perp, has := hedgeFutures[norm]; has {
			legs = append(legs, HedgeLeg{
				Asset:  c.sym,
				Amount: c.amt,
				Perp:   perp,
			})
		}
	}
	return legs
}

// hedgeLeg picks the volatile leg of a position to hedge and the perp to use.
// It prefers the non-stable leg; if both are volatile it hedges symbol0.
func hedgeLeg(sym0, sym1 string, amt0, amt1 float64) (asset string, amount float64, perp string, ok bool) {
	legs := hedgeLegs(sym0, sym1, amt0, amt1)
	if len(legs) == 0 {
		return "", 0, "", false
	}
	return legs[0].Asset, legs[0].Amount, legs[0].Perp, true
}

// runAnalysis runs the fee-vs-volatility model over a pool, attaching implied
// volatility for the base asset when the IV source has it.
func runAnalysis(ctx context.Context, rp datasource.RawPool, iv datasource.ImpliedVolSource) analyzer.Result {
	in := analyzer.Input{
		FeeTier:        rp.FeeTier,
		TVLUSD:         rp.TVLUSD,
		Volume24hUSD:   rp.Volume24hUSD,
		Closes:         rp.Closes,
		PeriodsPerYear: rp.PeriodsPerYear,
		Bars:           rp.OHLC,
	}
	if iv != nil {
		if v, ok := iv.ImpliedVol(ctx, rp.BaseSymbol); ok {
			in.ImpliedVol = v
			in.HasImplied = true
		}
	}
	return analyzer.Analyze(in)
}
