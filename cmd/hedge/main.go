// Command hedge reads an LP position from the chain and syncs a Binance
// futures short to match it. This first version handles only the WETH leg
// and always runs in dry-run mode, so it never sends a real order.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"

	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/filipekdick/lp-tracker-go/internal/lp"
)

// Configuration that does not change between runs
const (
	nfpmAddress    = "0x827922686190790b37229fd06084350E74485b72"
	factoryAddress = "0x5e7BB104d84c7CB9B682AaC2F3d509f5F406809A"

	defaultTokenID = 71002035

	//ethSymbol = "ETHUSDT" // the Binance perpetual we short against WETH
	minChange = 0.001 // smallest change in size worth a trade
	dryRun    = false // true = print only, never send a real order
)

func main() {
	//1. Load .env into the process environment. A missing file is not fatal:
	// on the server the values might already be set in the environment.
	if err := godotenv.Load(); err != nil {
		log.Println("note: no file found, relying on existing environment")
	}

	//2.Read what we need and stop early if anything is missing.
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Fatalf("Rpc url is not set")
	}
	apiKey := os.Getenv("BINANCE_TESTNET_API_KEY")
	apiSecret := os.Getenv("BINANCE_TESTNET_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		log.Fatal("missing Binance testnet keys in .env")
	}

	//3. Decide which position to read.
	tokenID := parseTokenID()

	//4. Connect to the Base chain
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to RPC: %v", err)
	}
	defer client.Close()

	//5.Build the two sides: the LP reader and the Binance client.
	nfpmAddrEnv := os.Getenv("NFPM_ADDRESS")
	if nfpmAddrEnv == "" {
		nfpmAddrEnv = nfpmAddress
	}
	reader := lp.NewReader(client, nfpmAddrEnv, factoryAddress)
	bn := binance.Connect(apiKey, apiSecret)

	ctx := context.Background()

	//Align our clock with Binance to avoid timestamp (-1021) rejections
	if err := bn.SyncTime(ctx); err != nil {
		log.Fatalf("could not sync time with Binance: %v", err)
	}

	//6. Read the LP position from the chain.
	report, err := reader.ReadPosition(tokenID)
	if err != nil {
		log.Fatalf("failed to read LP position %d: %v", tokenID, err)
	}
	log.Printf("LP %d: %s=%.6f  %s=%.6f  (in range: %t)",
		report.TokenID, report.Symbol0, report.Amount0,
		report.Symbol1, report.Amount1, report.InRange)

	//7. Hedge each leg. A leg the LP does not hold, or a symbol the venue does not list
	// is skipped without stopping the other leg.
	hedgeLeg(ctx, bn, report, "WETH", "ETHUSDT", dryRun)
	hedgeLeg(ctx, bn, report, "VIRTUAL", "VIRTUALUSDT", dryRun)

	log.Println("done")
}

// hedgeLeg syncs one Binance short to match one token in the LP position.
// It logs what it does and never returns an error, so a single failing or
// missing leg cannot stop the others.
func hedgeLeg(
	ctx context.Context,
	bn *binance.Client,
	r lp.PositionReport,
	lpSymbol, futuresSymbol string,
	dryRun bool,
) {
	amount, found := amountForSymbol(r, lpSymbol)
	if !found {
		log.Printf("%s: LP has no %s leg, skipping", futuresSymbol, lpSymbol)
		return
	}
	log.Printf("%s in LP: %.6f -> target short on %s: %.6f",
		lpSymbol, amount, futuresSymbol, amount)

	err := bn.SyncShort(ctx, futuresSymbol, amount, minChange, dryRun)
	if errors.Is(err, binance.ErrSymbolNotFound) {
		log.Printf("%s: not listed on this venue (likely testnet), skipping leg", futuresSymbol)
		return
	}
	if err != nil {
		log.Printf("%s: hedge failed: %v", futuresSymbol, err)
	}
}

// parseTokenID reads the token ID from the first command-line argument, or returns the default
// when none is given.
func parseTokenID() int64 {
	if len(os.Args) < 2 {
		log.Printf("no token ID given, using default %d", defaultTokenID)
		return defaultTokenID
	}
	id, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		log.Fatalf("invalid token ID %q: %v", os.Args[1], err)
	}
	return id
}

// amountForSymbol returns the amount of a given tokey symbol held in the
// position. The second return value is false when the position does not
// contain that symbol at all.
func amountForSymbol(r lp.PositionReport, symbol string) (float64, bool) {
	if r.Symbol0 == symbol {
		return r.Amount0, true
	}
	if r.Symbol1 == symbol {
		return r.Amount1, true
	}
	return 0, false
}
