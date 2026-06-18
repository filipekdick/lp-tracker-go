package lp

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
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
	NFPMAddress string
	PoolAddress string
	Symbol0     string
	Symbol1     string
	Amount0     float64
	Amount1     float64
	TickLower   int64
	TickUpper   int64
	TickNow     int64
	InRange     bool
	TokensOwed0 float64
	TokensOwed1 float64
}

// Reader reades a LP position from one protocol, given its contract addresses.
type Reader struct {
	client      *ethclient.Client
	nfpmAddrs   []common.Address
	factoryAddr common.Address
}

// NewReader builds a Reader for a connnected client and a protocol's addresses.
func NewReader(client *ethclient.Client, nfpmAddr, factoryAddr string) *Reader {
	var addrs []common.Address
	for _, part := range strings.Split(nfpmAddr, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			addrs = append(addrs, common.HexToAddress(part))
		}
	}

	// Default popular Aerodrome NFPM addresses
	popular := []string{
		"0x827922686190790b37229fd06084350E74485b72",
		"0xe1f8cd9AC4e4A65F54f38a5CdAfCA44f6dD68b53",
	}
	seen := make(map[common.Address]bool)
	for _, addr := range addrs {
		seen[addr] = true
	}
	for _, popStr := range popular {
		popAddr := common.HexToAddress(popStr)
		if !seen[popAddr] {
			addrs = append(addrs, popAddr)
			seen[popAddr] = true
		}
	}

	return &Reader{
		client:      client,
		nfpmAddrs:   addrs,
		factoryAddr: common.HexToAddress(factoryAddr),
	}
}

// ReadPosition reads all on-chain data for a token ID and computes the amounts.
func (r *Reader) ReadPosition(tokenID int64) (PositionReport, error) {
	var position struct {
		Nonce                    *big.Int
		Operator                 common.Address
		Token0                   common.Address
		Token1                   common.Address
		TickSpacing              *big.Int
		TickLower                *big.Int
		TickUpper                *big.Int
		Liquidity                *big.Int
		FeeGrowthInside0LastX128 *big.Int
		FeeGrowthInside1LastX128 *big.Int
		TokensOwed0              *big.Int
		TokensOwed1              *big.Int
	}
	var lastErr error
	var success bool
	var triedAddrs []string
	var successAddr common.Address

	for _, addr := range r.nfpmAddrs {
		triedAddrs = append(triedAddrs, addr.Hex())
		nfpm, err := aerodrome.NewNFPM(addr, r.client)
		if err != nil {
			lastErr = fmt.Errorf("binding NFPM at %s: %w", addr.Hex(), err)
			continue
		}
		pos, err := nfpm.Positions(nil, big.NewInt(tokenID))
		if err != nil {
			lastErr = fmt.Errorf("reading positions from %s: %w", addr.Hex(), err)
			continue
		}
		position = pos
		success = true
		successAddr = addr
		break
	}

	if !success {
		return PositionReport{}, fmt.Errorf(
			"token ID %d was not found on the tried NFPM contract(s) (%s). The position may live on a different NFPM deployment. Please set the NFPM_ADDRESS env var to the correct position-NFT contract (which can be found on Basescan under the NFT's 'Contract Address' field). Last error: %w",
			tokenID, strings.Join(triedAddrs, ", "), lastErr,
		)
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

	report := PositionReport{
		TokenID:     tokenID,
		NFPMAddress: successAddr.Hex(),
		PoolAddress: poolAddr.Hex(),
		Symbol0:     sym0,
		Symbol1:     sym1,
		Amount0:     format.TokenAmount(amount0, dec0),
		Amount1:     format.TokenAmount(amount1, dec1),
		TickLower:   tickLower,
		TickUpper:   tickUpper,
		TickNow:     tickNow,
		InRange:     tickNow >= tickLower && tickNow <= tickUpper,
		TokensOwed0: format.TokenAmount(position.TokensOwed0, dec0),
		TokensOwed1: format.TokenAmount(position.TokensOwed1, dec1),
	}

	// Simulate collect() to get real uncollected fees instead of static TokensOwed
	parsedABI, err := aerodrome.NFPMMetaData.GetAbi()
	if err == nil {
		maxUint128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
		data, err := parsedABI.Pack("collect", aerodrome.INonfungiblePositionManagerCollectParams{
			TokenId:    big.NewInt(tokenID),
			Recipient:  common.Address{},
			Amount0Max: maxUint128,
			Amount1Max: maxUint128,
		})
		if err == nil {
			msg := ethereum.CallMsg{
				To:   &successAddr,
				Data: data,
			}
			res, err := r.client.CallContract(context.Background(), msg, nil)
			if err == nil && len(res) >= 64 {
				out, err := parsedABI.Unpack("collect", res)
				if err == nil && len(out) == 2 {
					report.TokensOwed0 = format.TokenAmount(out[0].(*big.Int), dec0)
					report.TokensOwed1 = format.TokenAmount(out[1].(*big.Int), dec1)
				}
			}
		}
	}

	return report, nil
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
