package main

import (
	"context"
	"log"
	"os"
	"strconv"

	//"strings"

	"github.com/filipekdick/lp-tracker-go/internal/binance"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("could not load .env file: %v", err)
	}

	apiKey := os.Getenv("BINANCE_TESTNET_API_KEY")
	apiSecret := os.Getenv("BINANCE_TESTNET_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		log.Fatal("missing Binance testnet keys in .env")
	}

	client := binance.Connect(apiKey, apiSecret)

	serverTime, err := client.Ping(context.Background())
	if err != nil {
		log.Fatalf("ping failed: %v", err)
	}

	log.Printf("Connected to Binance testnet. Server time: %d", serverTime)

	balances, err := client.GetBalances(context.Background())
	if err != nil {
		log.Fatalf("reading balances failed: %v", err)
	}

	log.Println("Account balances (non-zero):")
	for _, b := range balances {
		amount, err := strconv.ParseFloat(b.Balance, 64)
		if err != nil {
			log.Printf(" could not parse balance for %s: %v", b.Asset, err)
			continue
		}
		if amount > 0 {
			log.Printf("%s: %s (available: %s", b.Asset, b.Balance, b.AvailableBalance)
		}
	}

	// positions, err := client.GetPositions(context.Background())
	// if err != nil {
	// 	log.Fatalf("reading positions failed: %v", err)
	// }

	err = client.SyncShort(context.Background(), "ETHUSDT", 0.1, 0.001, true)
	if err != nil {
		log.Fatalf("sync failed: %v", err)
	}

	open, err := client.GetOpenPositions(context.Background())
	if err != nil {
		log.Fatalf("reading positions failed: %v", err)
	}

	if len(open) == 0 {
		log.Println("No open positions.")
	}
	for _, p := range open {
		side := "LONG"
		amount := p.Size
		if p.Size < 0 {
			side = "SHORT"
			amount = -p.Size
		}
		log.Printf("%s: %s %.3f | entry %.2f mark %.2f PnL %.2f",
			p.Symbol, side, amount, p.EntryPrice, p.MarkPrice, p.UnrealizedPnL)
	}
}
