package v3math

import (
	"math/big"
)

// GetAmountsForLiquidity returns the amounts of token0 and token1 that a
// position with the given liquidity currently contains, given the current
// pool sqrt-price and the position's tick range.
func GetAmountsForLiquidity(
	sqrtPriceX96 *big.Int,
	tickLower int64,
	tickUpper int64,
	liquidity *big.Int,
) (amount0 *big.Int, amount1 *big.Int) {
	sqrtLower := GetSqrtRatioAtTick(tickLower)
	sqrtUpper := GetSqrtRatioAtTick(tickUpper)

	if sqrtPriceX96.Cmp(sqrtLower) <= 0 {
		//Price below range: position is entirely in token0
		amount0 = getAmount0(sqrtLower, sqrtUpper, liquidity)
		amount1 = big.NewInt(0)
	} else if sqrtPriceX96.Cmp(sqrtUpper) >= 0 {
		// Price above range: position is entirely in token1.
		amount0 = big.NewInt(0)
		amount1 = getAmount1(sqrtLower, sqrtUpper, liquidity)
	} else {
		//In range: position is split between the two tokens
		amount0 = getAmount0(sqrtPriceX96, sqrtUpper, liquidity)
		amount1 = getAmount1(sqrtLower, sqrtPriceX96, liquidity)
	}
	return amount0, amount1
}

// getAmount0 = (L << 96) * (sqrtB - sqrtA) / (sqrtA * sqrtB)
func getAmount0(sqrtA, sqrtB, liquidity *big.Int) *big.Int {
	if sqrtA.Cmp(sqrtB) > 0 {
		sqrtA, sqrtB = sqrtB, sqrtA
	}
	numerator := new(big.Int).Lsh(liquidity, 96)
	numerator.Mul(numerator, new(big.Int).Sub(sqrtB, sqrtA))
	denominator := new(big.Int).Mul(sqrtA, sqrtB)
	if denominator.Sign() == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(numerator, denominator)
}

// getAmount1 = L * (sqrtB - sqrtA) >> 96
func getAmount1(sqrtA, sqrtB, liquidity *big.Int) *big.Int {
	if sqrtA.Cmp(sqrtB) > 0 {
		sqrtA, sqrtB = sqrtB, sqrtA
	}
	diff := new(big.Int).Sub(sqrtB, sqrtA)
	return new(big.Int).Rsh(new(big.Int).Mul(liquidity, diff), 96)
}
