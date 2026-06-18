package binance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/filipekdick/lp-tracker-go/internal/hedger"
)

// Client wraps the Binance futures client so the rest of our
// program does not need to know the library's internal details.
type Client struct {
	futures *futures.Client
}

// Position is one open futures position in our simple form.
// Position is one open futures position in our own simple form.
type Position struct {
	Symbol           string  `json:"symbol"`
	Size             float64 `json:"size"` // negative = short, positive = long
	EntryPrice       float64 `json:"entryPrice"`
	MarkPrice        float64 `json:"markPrice"`
	UnrealizedPnL    float64 `json:"unrealizedPnl"`
	LiquidationPrice float64 `json:"liquidationPrice"`
}

// Side tells Binance whether we are buying or selling.
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// ErrSymbolNotFound means the venue (for example the testnet) does not
// list the requested trading symbol.
var ErrSymbolNotFound = errors.New("symbol not listed on this venue")

// Connect creates a Binance futures client pointed at the testnet.
func Connect(apiKey, apiSecret string) *Client {
	futures.UseTestnet = true
	fc := futures.NewClient(apiKey, apiSecret)
	return &Client{futures: fc}
}

// Ping checks that we can reach Binance and returns the server time.
func (c *Client) Ping(ctx context.Context) (int64, error) {
	serverTime, err := c.futures.NewServerTimeService().Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("Could not reach Binance: %w", err)
	}
	return serverTime, nil
}

// SyncTime aligns our clock with Binance's server time. Signed requests
// rejected with error -1021 if our timestamp drifts too far from theirs,
// happens on clould machines. Call this once after Connect.
func (c *Client) SyncTime(ctx context.Context) error {
	_, err := c.futures.NewServerTimeService().Do(ctx)
	if err != nil {
		return fmt.Errorf("syncing time with bincance: %w", err)
	}
	return nil
}

// parseFloatOrZero parses a Binance numeric string, returning 0 if it can't.
func parseFloatOrZero(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// GetQuantityPrecision returns how many decimal place Binance allows
// for order quantities on this symbol
func (c *Client) GetQuantityPrecision(ctx context.Context, symbol string) (int, error) {
	info, err := c.futures.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("Could not read exchange info: %w", err)
	}
	for _, s := range info.Symbols {
		if s.Symbol == symbol {
			return s.QuantityPrecision, nil
		}
	}
	return 0, fmt.Errorf("symbol %s not found in exchange info: %w", symbol, ErrSymbolNotFound)
}

// GetBalances reads all asset balances in the futures acccount.
func (c *Client) GetBalances(ctx context.Context) ([]*futures.Balance, error) {
	balances, err := c.futures.NewGetBalanceService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not read balances: %w", err)
	}
	return balances, nil
}

// GetPositions reads all positions, including ones with zero size.
func (c *Client) GetPositions(ctx context.Context) ([]*futures.PositionRisk, error) {
	positions, err := c.futures.NewGetPositionRiskService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not read positions: %w", err)
	}
	return positions, nil
}

// GetPositionSize returns the signed size of the position for one symbol.
// Negative means short, positive means long, zero means no open positions.
func (c *Client) GetPositionSize(ctx context.Context, symbol string) (float64, error) {
	positions, err := c.GetPositions(ctx)
	if err != nil {
		return 0, err
	}
	for _, p := range positions {
		if p.Symbol != symbol {
			continue
		}
		size, err := strconv.ParseFloat(p.PositionAmt, 64)
		if err != nil {
			return 0, fmt.Errorf("could not parse size for %s: %w", symbol, err)
		}
		return size, nil
	}
	return 0, nil
}

// GetOpenPositions returns every position with a non-zero size.
func (c *Client) GetOpenPositions(ctx context.Context) ([]Position, error) {
	raw, err := c.GetPositions(ctx)
	if err != nil {
		return nil, err
	}

	var open []Position
	for _, p := range raw {
		size, err := strconv.ParseFloat(p.PositionAmt, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse size for %s: %w", p.Symbol, err)
		}
		if size != 0 {
			open = append(open, Position{
				Symbol:           p.Symbol,
				Size:             size,
				EntryPrice:       parseFloatOrZero(p.EntryPrice),
				MarkPrice:        parseFloatOrZero(p.MarkPrice),
				UnrealizedPnL:    parseFloatOrZero(p.UnRealizedProfit),
				LiquidationPrice: parseFloatOrZero(p.LiquidationPrice),
			})
		}
	}
	return open, nil
}

// PlaceMarketOrder places a market order. When dryRun is true it validates
// the order and prints what it would do, without sending anything to Binance.
// ReduceOnly orders can only shrink or close a position, never flip it.
func (c *Client) PlaceMarketOrder(
	ctx context.Context,
	symbol string,
	side Side,
	quantity string,
	reduceOnly bool,
	dryRun bool) (*futures.CreateOrderResponse, error) {
	//Pre-flight checks: refuse obviously bad orders before they reach Binance.
	if symbol == "" {
		return nil, fmt.Errorf("order rejected: symbol is empty")
	}

	qty, err := strconv.ParseFloat(quantity, 64)
	if err != nil {
		return nil, fmt.Errorf("order rejected: quantity %q is not a number: %w", quantity, err)
	}
	if qty <= 0 {
		return nil, fmt.Errorf("order rejected: quantity must be positive, got %v", qty)
	}

	var binanceSide futures.SideType
	switch side {
	case SideBuy:
		binanceSide = futures.SideTypeBuy
	case SideSell:
		binanceSide = futures.SideTypeSell
	default:
		return nil, fmt.Errorf("order rejected: unknow side %q", side)
	}

	if dryRun {
		log.Printf("[DRY RUN] would place MARKET %s: %s %s (reduceOnly=%t, nothing sent)", side, quantity, symbol, reduceOnly)
		return nil, nil
	}

	order, err := c.futures.NewCreateOrderService().
		Symbol(symbol).
		Side(binanceSide).
		Type(futures.OrderTypeMarket).
		Quantity(quantity).
		ReduceOnly(reduceOnly).
		NewOrderResponseType(futures.NewOrderRespTypeRESULT).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("placing order failed: %w", err)
	}
	log.Printf("[LIVE]Placed MARKE %s: %s %s (reduceOnly=%t) -> orderID %d, filled %s @avg %s, status %s",
		side, quantity, symbol, reduceOnly, order.OrderID, order.CumQty, order.AvgPrice, order.Status)
	return order, nil
}

// SynchShort maker our short in 'simbol' match 'TargetShort' base-asset units.
// A positive targetShort means "be short this much"; zero means "be flat".
// It only trades when the gap is at least 'minChange', to avoid churn on noise.
func (c *Client) SyncShort(
	ctx context.Context, symbol string, targetShort,
	minChange float64, dryRun bool, strategy *hedger.Strategy) error {
	if targetShort < 0 {
		return fmt.Errorf("targetShort must be zero or positive, got %v", targetShort)
	}

	current, err := c.GetPositionSize(ctx, symbol)
	if err != nil {
		return err
	}

	desired := -targetShort    //a short is stored as a negative size
	delta := desired - current //signed gap: how far, and which way, to move

	if math.Abs(delta) < minChange {
		log.Printf("%s: within tolerance (short %.2f, target %.2f), no order", symbol, -current, targetShort)
		return nil
	}

	side := SideSell //delta <0 means we need a bigger short, so sell more
	reduceOnly := false
	if delta > 0 { //delta > 0 means we are short too short, so buy some back
		side = SideBuy
		reduceOnly = true
	}

	precision, err := c.GetQuantityPrecision(ctx, symbol)
	if err != nil {
		return err
	}

	// 1. Calculate drift percentage
	driftPct := 0.0
	if targetShort > 0 {
		driftPct = delta / targetShort
	}

	// 2. Determine Action
	var action hedger.Action
	var reason string
	if strategy != nil {
		action, reason = strategy.Decide(driftPct)
	} else {
		action, reason = hedger.ActionMarket, "no strategy provided"
	}

	quantity := strconv.FormatFloat(math.Abs(delta), 'f', precision, 64)

	// Fetch Min Notional and Mark Price
	minNotional, _ := c.GetMinNotional(ctx, symbol)
	openPositions, _ := c.GetOpenPositions(ctx)
	var markPrice float64
	for _, p := range openPositions {
		if p.Symbol == symbol {
			markPrice = p.MarkPrice
			break
		}
	}
	if markPrice == 0 {
		// fallback to market price
		prices, err := c.futures.NewListPricesService().Symbol(symbol).Do(ctx)
		if err == nil && len(prices) > 0 {
			markPrice = parseFloatOrZero(prices[0].Price)
		}
	}

	qtyFloat, _ := strconv.ParseFloat(quantity, 64)
	if qtyFloat == 0 || (markPrice > 0 && minNotional > 0 && qtyFloat*markPrice < minNotional) {
		log.Printf("%s: gap %.4f below min notional %.2f, skipping", symbol, qtyFloat, minNotional)
		return nil
	}

	switch action {
	case hedger.ActionNone:
		log.Printf("%s: %s (short %.2f, target %.2f)", symbol, reason, -current, targetShort)
		return nil

	case hedger.ActionLimit:
		// Check existing open limit orders
		openOrders, err := c.GetOpenOrders(ctx, symbol)
		if err == nil && len(openOrders) > 0 {
			// for simplicity, cancel existing to replace
			c.CancelAllOrders(ctx, symbol)
		}
		priceStr := strconv.FormatFloat(markPrice, 'f', 4, 64) // basic precision
		log.Printf("%s: placing LIMIT order (%s), driftPct: %.4f", symbol, reason, driftPct)
		_, err = c.PlaceLimitOrder(ctx, symbol, side, quantity, priceStr, reduceOnly, dryRun)
		if err != nil && strings.Contains(err.Error(), "-4131") {
			log.Printf("%s: limit price outside PERCENT_PRICE band, skipping this cycle", symbol)
			return nil
		}
		return err

	case hedger.ActionMarket:
		// Cancel existing limit orders to avoid over-hedging
		c.CancelAllOrders(ctx, symbol)
		log.Printf("%s: placing MARKET order (%s), driftPct: %.4f", symbol, reason, driftPct)
		_, err = c.PlaceMarketOrder(ctx, symbol, side, quantity, reduceOnly, dryRun)
		return err
	}

	return nil
}

// GetOpenOrders lists active limit orders for the symbol.
func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]*futures.Order, error) {
	return c.futures.NewListOpenOrdersService().Symbol(symbol).Do(ctx)
}

// CancelAllOrders cancels all active orders for the symbol.
func (c *Client) CancelAllOrders(ctx context.Context, symbol string) error {
	return c.futures.NewCancelAllOpenOrdersService().Symbol(symbol).Do(ctx)
}

// GetMinNotional fetches the MIN_NOTIONAL filter for the symbol.
func (c *Client) GetMinNotional(ctx context.Context, symbol string) (float64, error) {
	info, err := c.futures.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("could not read exchange info: %w", err)
	}
	for _, s := range info.Symbols {
		if s.Symbol == symbol {
			for _, f := range s.Filters {
				if f["filterType"] == "MIN_NOTIONAL" {
					if val, ok := f["notional"].(string); ok {
						return strconv.ParseFloat(val, 64)
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("MIN_NOTIONAL not found for %s", symbol)
}

// PlaceLimitOrder places a post-only limit order.
func (c *Client) PlaceLimitOrder(
	ctx context.Context,
	symbol string,
	side Side,
	quantity string,
	price string,
	reduceOnly bool,
	dryRun bool) (*futures.CreateOrderResponse, error) {

	if dryRun {
		log.Printf("[DRY RUN] would place LIMIT %s: %s %s @ %s (reduceOnly=%t)", side, quantity, symbol, price, reduceOnly)
		return nil, nil
	}

	var binanceSide futures.SideType
	switch side {
	case SideBuy:
		binanceSide = futures.SideTypeBuy
	case SideSell:
		binanceSide = futures.SideTypeSell
	}

	order, err := c.futures.NewCreateOrderService().
		Symbol(symbol).
		Side(binanceSide).
		Type(futures.OrderTypeLimit).
		TimeInForce(futures.TimeInForceTypeGTX). // Post Only
		Quantity(quantity).
		Price(price).
		ReduceOnly(reduceOnly).
		NewOrderResponseType(futures.NewOrderRespTypeRESULT).
		Do(ctx)

	if err != nil {
		return nil, err
	}
	log.Printf("[LIVE] Placed LIMIT %s: %s %s @ %s -> orderID %d, status %s", side, quantity, symbol, price, order.OrderID, order.Status)
	return order, nil
}
