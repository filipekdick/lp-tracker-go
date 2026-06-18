package lp

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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

	// UncollectedFees0/1 are the fees currently owed to the position but not yet
	// withdrawn — the live, claimable balance from the simulated collect() call.
	UncollectedFees0 float64
	UncollectedFees1 float64
	// CollectedFees0/1 are the cumulative fees already withdrawn over the
	// position's lifetime, reconstructed from event logs (best-effort; 0 when
	// the history could not be read).
	CollectedFees0 float64
	CollectedFees1 float64
}

// Reader reades a LP position from one protocol, given its contract addresses.
type Reader struct {
	client      *ethclient.Client
	nfpmAddrs   []common.Address
	factoryAddr common.Address
	// feesFromBlock bounds the Collect/DecreaseLiquidity log scan. 0 means from
	// genesis, which is fine on Alchemy for an indexed single-position query;
	// set LP_FEES_FROM_BLOCK on RPCs that cap the block range per eth_getLogs.
	feesFromBlock uint64
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

	var fromBlock uint64
	if v := os.Getenv("LP_FEES_FROM_BLOCK"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			fromBlock = n
		}
	}

	return &Reader{
		client:        client,
		nfpmAddrs:     addrs,
		factoryAddr:   common.HexToAddress(factoryAddr),
		feesFromBlock: fromBlock,
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
		// Start from the position's static tokensOwed; the collect() simulation
		// below refines this to the live claimable balance when it succeeds.
		UncollectedFees0: format.TokenAmount(position.TokensOwed0, dec0),
		UncollectedFees1: format.TokenAmount(position.TokensOwed1, dec1),
	}

	// Simulate collect() to get real uncollected fees instead of static tokensOwed
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
					report.UncollectedFees0 = format.TokenAmount(out[0].(*big.Int), dec0)
					report.UncollectedFees1 = format.TokenAmount(out[1].(*big.Int), dec1)
				}
			}
		}
	}

	// Historical collected fees from event logs. Non-fatal: a log-query failure
	// (RPC range limit, missing binding) leaves collected fees at 0 but still
	// returns a valid report with the uncollected figures above.
	if c0, c1, err := r.collectedFeesAt(successAddr, tokenID, dec0, dec1); err != nil {
		log.Printf("[lp] collected-fee history unavailable for token %d: %v", tokenID, err)
	} else {
		report.CollectedFees0 = c0
		report.CollectedFees1 = c1
	}

	return report, nil
}

// CollectedFees reconstructs the total fees already withdrawn for a position
// over its lifetime by summing the NFPM Collect events for this tokenID across
// every configured NFPM (a tokenID is unique to one of them, so the others
// contribute nothing). Returns human-readable amounts using the pool's token
// decimals.
func (r *Reader) CollectedFees(tokenID int64, dec0, dec1 uint8) (float64, float64, error) {
	var total0, total1 float64
	for _, addr := range r.nfpmAddrs {
		c0, c1, err := r.collectedFeesAt(addr, tokenID, dec0, dec1)
		if err != nil {
			return 0, 0, err
		}
		total0 += c0
		total1 += c1
	}
	return total0, total1, nil
}

// collectedFeesAt sums collected fees for one NFPM contract. Uniswap-V3-style
// Collect events report what was pulled out of the position's tokensOwed
// balance, which bundles accrued fees with any principal that decreaseLiquidity
// previously moved into tokensOwed. To isolate fees we subtract the principal
// removed via DecreaseLiquidity:
//
//	collected_fees = Σ Collect.amount − Σ DecreaseLiquidity.amount   (clamped ≥ 0)
//
// Caveat: this assumes decreased principal is eventually collected (the usual
// case). If liquidity was decreased but not yet collected, that principal still
// sits in the live uncollected balance, so the collected/uncollected split can
// be momentarily off by the pending amount until it is collected.
func (r *Reader) collectedFeesAt(addr common.Address, tokenID int64, dec0, dec1 uint8) (float64, float64, error) {
	nfpm, err := aerodrome.NewNFPM(addr, r.client)
	if err != nil {
		return 0, 0, fmt.Errorf("binding NFPM at %s: %w", addr.Hex(), err)
	}

	opts := &bind.FilterOpts{Start: r.feesFromBlock, Context: context.Background()}
	id := []*big.Int{big.NewInt(tokenID)}

	collect0, collect1 := new(big.Int), new(big.Int)
	cIt, err := nfpm.FilterCollect(opts, id)
	if err != nil {
		return 0, 0, fmt.Errorf("filtering Collect events: %w", err)
	}
	defer cIt.Close()
	for cIt.Next() {
		collect0.Add(collect0, cIt.Event.Amount0)
		collect1.Add(collect1, cIt.Event.Amount1)
	}
	if err := cIt.Error(); err != nil {
		return 0, 0, fmt.Errorf("iterating Collect events: %w", err)
	}

	decrease0, decrease1 := new(big.Int), new(big.Int)
	dIt, err := nfpm.FilterDecreaseLiquidity(opts, id)
	if err != nil {
		return 0, 0, fmt.Errorf("filtering DecreaseLiquidity events: %w", err)
	}
	defer dIt.Close()
	for dIt.Next() {
		decrease0.Add(decrease0, dIt.Event.Amount0)
		decrease1.Add(decrease1, dIt.Event.Amount1)
	}
	if err := dIt.Error(); err != nil {
		return 0, 0, fmt.Errorf("iterating DecreaseLiquidity events: %w", err)
	}

	f0, f1 := collectedFeeAmounts(collect0, collect1, decrease0, decrease1, dec0, dec1)
	return f0, f1, nil
}

// collectedFeeAmounts isolates collected fees from total Collect withdrawals by
// subtracting principal removed via DecreaseLiquidity, clamping at zero, and
// converting to human-readable amounts. Summation stays in big.Int; the float
// conversion happens only at the end (display-only).
func collectedFeeAmounts(collect0, collect1, decrease0, decrease1 *big.Int, dec0, dec1 uint8) (float64, float64) {
	fee0 := new(big.Int).Sub(collect0, decrease0)
	if fee0.Sign() < 0 {
		fee0 = big.NewInt(0)
	}
	fee1 := new(big.Int).Sub(collect1, decrease1)
	if fee1.Sign() < 0 {
		fee1 = big.NewInt(0)
	}
	return format.TokenAmount(fee0, dec0), format.TokenAmount(fee1, dec1)
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
