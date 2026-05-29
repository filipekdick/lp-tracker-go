package format

import (
	"math/big"
)

// TokenAmount converts a raw on-chain integer amount into a human-readable
// float64, dividing 10^decimals. Note: float64 loses precision for every
// large numbers, so this is for display only, never further math
func TokenAmount(raw *big.Int, decimals uint8) float64 {
	if raw == nil {
		return 0
	}
	//Build 10^decimals as a big.Float
	divisor := new(big.Float).SetInt(
		new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil),
	)
	rawFloat := new(big.Float).SetInt(raw)
	result := new(big.Float).Quo(rawFloat, divisor)

	f, _ := result.Float64()
	return f
}
