package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/filipekdick/lp-tracker-go/internal/lp"
	"github.com/joho/godotenv"
)

const (
	nfpmAddress    = "0x827922686190790b37229fd06084350E74485b72"
	factoryAddress = "0x5e7BB104d84c7CB9B682AaC2F3d509f5F406809A"
	tokenIDValue   = 71002035
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

	nfpmAddrEnv := os.Getenv("NFPM_ADDRESS")
	if nfpmAddrEnv == "" {
		nfpmAddrEnv = nfpmAddress
	}
	reader := lp.NewReader(client, nfpmAddrEnv, factoryAddress)
	report, err := reader.ReadPosition(tokenID)
	if err != nil {
		log.Fatalf("failed to read position: %v", err)
	}

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

// printReport displays a PositionReport in a readable format.
func printReport(r lp.PositionReport) {
	fmt.Printf("Position %d (%s/%s)\n", r.TokenID, r.Symbol0, r.Symbol1)
	fmt.Printf("  tick range:  [%d, %d], current %d\n", r.TickLower, r.TickUpper, r.TickNow)
	if r.InRange {
		fmt.Println("  status:      IN RANGE")
	} else {
		fmt.Println("  status:      OUT OF RANGE")
	}
	fmt.Printf("  %s: %.2f\n", r.Symbol0, r.Amount0)
	fmt.Printf("  %s: %.2f\n", r.Symbol1, r.Amount1)
}
