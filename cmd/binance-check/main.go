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

	symbol := "ETHUSDT"
	size, err := client.GetPositionSize(context.Background(), symbol)
	if err != nil {
		log.Fatalf("reading position failed: %v", err)
	}

	switch {
	case size < 0:
		log.Printf("%s position: SHORT %.2f", symbol, -size)
	case size > 0:
		log.Printf("%s position: LONG %.2f", symbol, size)
	default:
		log.Printf("%s position: flat (none open)", symbol)
	}
}
