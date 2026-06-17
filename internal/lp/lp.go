package lp

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/filipekdick/lp-tracker-go/contracts/aerodrome"
	"github.com/filipekdick/lp-tracker-go/contracts/erc20"
	"github.com/filipekdick/lp-tracker-go/contracts/factory"
	"github.com/filipekdick/lp-tracker-go/contracts/pool"
	"github.com/filipekdick/lp-tracker-go/internal/format"
	"github.com/filipekdick/lp-tracker-go/internal/v3math"
)

// PositionReport bundles everything we want to know about a position.
type PositionReport struct {
	TokenID     int64
	PoolAddress string
	Symbol0     string
	Symbol1     string
	Amount0     float64
	Amount1     float64
	TickLower   int64
	TickUpper   int64
	TickNow     int64
	InRange     bool
}

// Reader reades a LP position from one protocol, given its contract addresses.
type Reader struct {
	client      *ethclient.Client
	nfpmAddr    common.Address
	factoryAddr common.Address
}

// NewReader builds a Reader for a connnected client and a protocol's addresses.
func NewReader(client *ethclient.Client, nfpmAddr, factoryAddr string) *Reader {
	return &Reader{
		client:      client,
		nfpmAddr:    common.HexToAddress(nfpmAddr),
		factoryAddr: common.HexToAddress(factoryAddr),
	}
}

// ReadPosition reads all on-chain data for a token ID and computes the amounts.
func (r *Reader) ReadPosition(tokenID int64) (PositionReport, error) {
	nfpm, err := aerodrome.NewNFPM(r.nfpmAddr, r.client)
	if err != nil {
		return PositionReport{}, fmt.Errorf("binding NFPM: %w", err)
	}

	position, err := nfpm.Positions(nil, big.NewInt(tokenID))
	if err != nil {
		return PositionReport{}, fmt.Errorf("reading position %d: %w", tokenID, err)
	}

	poolAddr, err := r.findPool(position.Token0, position.Token1, position.TickSpacing)
	if err != nil {
		return PositionReport{}, err
	}

	poolContract, err := pool.NewPool(poolAddr, r.client)
	if err != nil {
		return PositionReport{}, fmt.Errorf("binding pool: %w", err)
	}
	slot0, err := poolContract.Slot0(nil)
	if err != nil {
		return PositionReport{}, fmt.Errorf("reading slot0: %w", err)
	}

	sym0, dec0, err := tokenMeta(r.client, position.Token0)
	if err != nil {
		return PositionReport{}, err
	}
	sym1, dec1, err := tokenMeta(r.client, position.Token1)
	if err != nil {
		return PositionReport{}, err
	}

	amount0, amount1 := v3math.GetAmountsForLiquidity(
		slot0.SqrtPriceX96,
		position.TickLower.Int64(),
		position.TickUpper.Int64(),
		position.Liquidity,
	)

	tickNow := slot0.Tick.Int64()
	tickLower := position.TickLower.Int64()
	tickUpper := position.TickUpper.Int64()

	return PositionReport{
		TokenID:     tokenID,
		PoolAddress: poolAddr.Hex(),
		Symbol0:     sym0,
		Symbol1:     sym1,
		Amount0:     format.TokenAmount(amount0, dec0),
		Amount1:     format.TokenAmount(amount1, dec1),
		TickLower:   tickLower,
		TickUpper:   tickUpper,
		TickNow:     tickNow,
		InRange:     tickNow >= tickLower && tickNow <= tickUpper,
	}, nil
}

// findPool derives the pool address for a position from the factory
func (r *Reader) findPool(token0, token1 common.Address, tickSpacing *big.Int) (common.Address, error) {
	factoryContract, err := factory.NewFactory(r.factoryAddr, r.client)
	if err != nil {
		return common.Address{}, fmt.Errorf("binding factory: %w", err)
	}
	poolAddr, err := factoryContract.GetPool(nil, token0, token1, tickSpacing)
	if err != nil {
		return common.Address{}, fmt.Errorf("getting pool: %w", err)
	}
	return poolAddr, nil
}

// tokenMeta reads an ERC20 token's symbol and decimals.
func tokenMeta(client *ethclient.Client, address common.Address) (string, uint8, error) {
	token, err := erc20.NewERC20(address, client)
	if err != nil {
		return "", 0, fmt.Errorf("binding token %s: %w", address.Hex(), err)
	}
	symbol, err := token.Symbol(nil)
	if err != nil {
		return "", 0, fmt.Errorf("reading symbol for %s: %w", address.Hex(), err)
	}
	decimals, err := token.Decimals(nil)
	if err != nil {
		return "", 0, fmt.Errorf("reading decimals for %s: %w", address.Hex(), err)
	}
	return symbol, decimals, nil
}
