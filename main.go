package main

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"

	"github.com/filipekdick/lp-tracker-go/contracts/aerodrome"
	"github.com/filipekdick/lp-tracker-go/contracts/erc20"
	"github.com/filipekdick/lp-tracker-go/contracts/factory"
	"github.com/filipekdick/lp-tracker-go/contracts/pool"
	"github.com/filipekdick/lp-tracker-go/internal/format"
	"github.com/filipekdick/lp-tracker-go/internal/v3math"
)

const (
	nfpmAddress    = "0x827922686190790b37229fd06084350E74485b72"
	factoryAddress = "0x5e7BB104d84c7CB9B682AaC2F3d509f5F406809A"
	tokenIDValue   = 71513742
)

// PositionReport bundles everything we want to display about a position.
type PositionReport struct {
	TokenID   int64
	Symbol0   string
	Symbol1   string
	Amount0   float64
	Amount1   float64
	TickLower int64
	TickUpper int64
	TickNow   int64
	InRange   bool
}

func main() {
	tokenID := parseTokenID()

	client := connect()
	defer client.Close()

	report := buildReport(client, tokenID)
	printReport(report)
}

// parseTokenID read the token ID from the first command-line argument,
// falling back to a default if none is given
func parseTokenID() int64 {
	if len(os.Args) < 2 {
		log.Printf("No token ID given, using default %d", tokenIDValue)
		return tokenIDValue
	}
	id, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		log.Fatalf("Invalid token ID: %q: :%v", os.Args[1], err)
	}
	return id
}

// connect loads the RPC URL from the environment and dials the chain.
func connect() *ethclient.Client {
	if err := godotenv.Load(); err != nil {
		log.Println("note: no .env file found, relying on existing environment")
	}
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Fatal("RPC_URL is not set; put it in your .env file")
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to RPC: %v", err)
	}
	return client
}

// findPool derives the pool address for a position from the factory.
func findPool(client *ethclient.Client, token0, token1 common.Address, tickSpacing *big.Int) common.Address {
	factoryContract, err := factory.NewFactory(common.HexToAddress(factoryAddress), client)
	if err != nil {
		log.Fatalf("failed to bind Factory: %v", err)
	}
	poolAddr, err := factoryContract.GetPool(nil, token0, token1, tickSpacing)
	if err != nil {
		log.Fatalf("failed to get pool: %v", err)
	}
	return poolAddr
}

// tokenMeta reads an ERC20 token's symbol and decimals.
func tokenMeta(client *ethclient.Client, address common.Address) (string, uint8) {
	token, err := erc20.NewERC20(address, client)
	if err != nil {
		log.Fatalf("failed to bind token %s: %v", address.Hex(), err)
	}
	symbol, err := token.Symbol(nil)
	if err != nil {
		log.Fatalf("failed to read symbol for %s: %v", address.Hex(), err)
	}
	decimals, err := token.Decimals(nil)
	if err != nil {
		log.Fatalf("failed to read decimals for %s: %v", address.Hex(), err)
	}
	return symbol, decimals
}

// buildReport reads all on-chain data for a token ID and computes the amounts.
func buildReport(client *ethclient.Client, tokenID int64) PositionReport {
	nfpm, err := aerodrome.NewNFPM(common.HexToAddress(nfpmAddress), client)
	if err != nil {
		log.Fatalf("failed to bind NFPM: %v", err)
	}

	position, err := nfpm.Positions(nil, big.NewInt(tokenID))
	if err != nil {
		log.Fatalf("failed to read position %d: %v", tokenID, err)
	}

	poolAddr := findPool(client, position.Token0, position.Token1, position.TickSpacing)

	poolContract, err := pool.NewPool(poolAddr, client)
	if err != nil {
		log.Fatalf("failed to bind Pool: %v", err)
	}
	slot0, err := poolContract.Slot0(nil)
	if err != nil {
		log.Fatalf("failed to read slot0: %v", err)
	}

	sym0, dec0 := tokenMeta(client, position.Token0)
	sym1, dec1 := tokenMeta(client, position.Token1)

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
		TokenID:   tokenID,
		Symbol0:   sym0,
		Symbol1:   sym1,
		Amount0:   format.TokenAmount(amount0, dec0),
		Amount1:   format.TokenAmount(amount1, dec1),
		TickLower: tickLower,
		TickUpper: tickUpper,
		TickNow:   tickNow,
		InRange:   tickNow >= tickLower && tickNow <= tickUpper,
	}
}

// printReport displays a PositionReport in a readable format.
func printReport(r PositionReport) {
	fmt.Printf("Position %d (%s/%s)\n", r.TokenID, r.Symbol0, r.Symbol1)
	fmt.Printf("  tick range:  [%d, %d], current %d\n", r.TickLower, r.TickUpper, r.TickNow)
	if r.InRange {
		fmt.Println("  status:      IN RANGE")
	} else {
		fmt.Println("  status:      OUT OF RANGE")
	}
	fmt.Printf("  %s: %.4f\n", r.Symbol0, r.Amount0)
	fmt.Printf("  %s: %.4f\n", r.Symbol1, r.Amount1)
}
