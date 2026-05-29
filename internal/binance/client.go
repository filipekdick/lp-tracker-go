package binance

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/adshao/go-binance/v2/futures"
)

// Client wraps the Binance futures client so the rest of our
// program does not need to know the library's internal details.
type Client struct {
	futures *futures.Client
}

// Side tells Binance whether we are buying or selling.
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

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
	return order, nil
}
