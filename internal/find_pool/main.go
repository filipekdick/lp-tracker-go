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
	"github.com/filipekdick/lp-tracker-go/contracts/factory"
)

const (
	nfpmAddress    = "0x827922686190790b37229fd06084350E74485b72"
	factoryAddress = "0x5e7BB104d84c7CB9B682AaC2F3d509f5F406809A"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run experiments/find_pool/main.go <tokenID>")
	}

	tokenIDValue, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		log.Fatalf("invalid token ID %q: %v", os.Args[1], err)
	}

	if err := godotenv.Load(".env"); err != nil {
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
	defer client.Close()

	nfpm, err := aerodrome.NewNFPM(common.HexToAddress(nfpmAddress), client)
	if err != nil {
		log.Fatalf("failed to bind NFPM: %v", err)
	}
	factoryContract, err := factory.NewFactory(common.HexToAddress(factoryAddress), client)
	if err != nil {
		log.Fatalf("failed to bind Factory: %v", err)
	}

	position, err := nfpm.Positions(nil, big.NewInt(tokenIDValue))
	if err != nil {
		log.Fatalf("failed to read position %d: %v", tokenIDValue, err)
	}

	poolAddr, err := factoryContract.GetPool(nil, position.Token0, position.Token1, position.TickSpacing)
	if err != nil {
		log.Fatalf("failed to get pool: %v", err)
	}

	fmt.Printf("Token ID:     %d\n", tokenIDValue)
	fmt.Printf("token0:       %s\n", position.Token0.Hex())
	fmt.Printf("token1:       %s\n", position.Token1.Hex())
	fmt.Printf("tickSpacing:  %s\n", position.TickSpacing.String())
	fmt.Printf("Pool address: %s\n", poolAddr.Hex())
}
