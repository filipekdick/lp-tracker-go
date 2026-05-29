package v3math

import (
	"math/big"
)

const (
	MinTick = -887272
	MaxTick = 887272
)

// GetSqrtRatioAtTick returns sqrt(1.0001^tick) * 2^96 as a big integer.
// This is the canonical Uniswap V3 TickMath.getSqrtRatioAtTick implementation,
// using a sequence of pre-computed magic constants.

func GetSqrtRatioAtTick(tick int64) *big.Int {
	if tick < MinTick || tick > MaxTick {
		panic("tick out of range")
	}

	absTick := tick
	if absTick < 0 {
		absTick = -absTick
	}

	var ratio *big.Int
	if absTick&0x1 != 0 {
		ratio = hexToBigInt("fffcb933bd6fad37aa2d162d1a594001")
	} else {
		ratio = hexToBigInt("100000000000000000000000000000000")
	}

	multiplyIfBitSet(ratio, absTick, 0x2, "fff97272373d413259a46990580e213a")
	multiplyIfBitSet(ratio, absTick, 0x4, "fff2e50f5f656932ef12357cf3c7fdcc")
	multiplyIfBitSet(ratio, absTick, 0x8, "ffe5caca7e10e4e61c3624eaa0941cd0")
	multiplyIfBitSet(ratio, absTick, 0x10, "ffcb9843d60f6159c9db58835c926644")
	multiplyIfBitSet(ratio, absTick, 0x20, "ff973b41fa98c081472e6896dfb254c0")
	multiplyIfBitSet(ratio, absTick, 0x40, "ff2ea16466c96a3843ec78b326b52861")
	multiplyIfBitSet(ratio, absTick, 0x80, "fe5dee046a99a2a811c461f1969c3053")
	multiplyIfBitSet(ratio, absTick, 0x100, "fcbe86c7900a88aedcffc83b479aa3a4")
	multiplyIfBitSet(ratio, absTick, 0x200, "f987a7253ac413176f2b074cf7815e54")
	multiplyIfBitSet(ratio, absTick, 0x400, "f3392b0822b70005940c7a398e4b70f3")
	multiplyIfBitSet(ratio, absTick, 0x800, "e7159475a2c29b7443b29c7fa6e889d9")
	multiplyIfBitSet(ratio, absTick, 0x1000, "d097f3bdfd2022b8845ad8f792aa5825")
	multiplyIfBitSet(ratio, absTick, 0x2000, "a9f746462d870fdf8a65dc1f90e061e5")
	multiplyIfBitSet(ratio, absTick, 0x4000, "70d869a156d2a1b890bb3df62baf32f7")
	multiplyIfBitSet(ratio, absTick, 0x8000, "31be135f97d08fd981231505542fcfa6")
	multiplyIfBitSet(ratio, absTick, 0x10000, "9aa508b5b7a84e1c677de54f3e99bc9")
	multiplyIfBitSet(ratio, absTick, 0x20000, "5d6af8dedb81196699c329225ee604")
	multiplyIfBitSet(ratio, absTick, 0x40000, "2216e584f5fa1ea926041bedfe98")
	multiplyIfBitSet(ratio, absTick, 0x80000, "48a170391f7dc42444e8fa2")

	if tick > 0 {
		maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
		ratio = new(big.Int).Div(maxUint256, ratio)
	}

	// Convert from Q128.128 to Q96 (shift right by 32, rounding up if any remainder).
	mod := new(big.Int).Mod(ratio, new(big.Int).Lsh(big.NewInt(1), 32))
	shifted := new(big.Int).Rsh(ratio, 32)
	if mod.Sign() != 0 {
		shifted.Add(shifted, big.NewInt(1))
	}
	return shifted
}

// hexToBigInt parses a hex string like "fffcb933..." into a big.Int.
func hexToBigInt(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("invalid hex string: " + s)
	}
	return n
}

// multiplyIfBitSet performs `ratio = (ratio * magic) >> 128` if `bit` is set in absTick.
// Modifies ratio in place.
func multiplyIfBitSet(ratio *big.Int, absTick int64, bit int64, hexMagic string) {
	if absTick&bit == 0 {
		return
	}
	magic := hexToBigInt(hexMagic)
	ratio.Mul(ratio, magic)
	ratio.Rsh(ratio, 128)
}
