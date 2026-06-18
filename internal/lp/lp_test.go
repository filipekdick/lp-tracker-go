package lp

import (
	"math"
	"math/big"
	"testing"
)

// bi parses a base-10 integer literal into a *big.Int for test fixtures.
func bi(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("bad big.Int literal: " + s)
	}
	return n
}

func TestCollectedFeeAmounts(t *testing.T) {
	const (
		wethDec uint8 = 18
		usdcDec uint8 = 6
	)
	tests := []struct {
		name           string
		c0, c1, d0, d1 *big.Int
		want0, want1   float64
	}{
		{
			name:  "fees only, no principal removed",
			c0:    bi("1500000000000000000"), // 1.5 WETH
			c1:    bi("3000000000"),          // 3000 USDC
			d0:    big.NewInt(0),
			d1:    big.NewInt(0),
			want0: 1.5,
			want1: 3000,
		},
		{
			name:  "collect bundles decreased principal",
			c0:    bi("5000000000000000000"), // 5 WETH withdrawn
			c1:    bi("1000000000"),          // 1000 USDC withdrawn
			d0:    bi("4000000000000000000"), // 4 WETH of that was principal
			d1:    bi("200000000"),           // 200 USDC of that was principal
			want0: 1.0,                       // 1 WETH of real fees
			want1: 800,                       // 800 USDC of real fees
		},
		{
			name:  "principal exceeds collect -> clamp to zero",
			c0:    bi("1000000000000000000"),
			c1:    bi("100000000"),
			d0:    bi("2000000000000000000"),
			d1:    bi("500000000"),
			want0: 0,
			want1: 0,
		},
		{
			name:  "never collected",
			c0:    big.NewInt(0),
			c1:    big.NewInt(0),
			d0:    big.NewInt(0),
			d1:    big.NewInt(0),
			want0: 0,
			want1: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture inputs to ensure the helper does not mutate them.
			c0Before := new(big.Int).Set(tt.c0)
			g0, g1 := collectedFeeAmounts(tt.c0, tt.c1, tt.d0, tt.d1, wethDec, usdcDec)
			if math.Abs(g0-tt.want0) > 1e-9 {
				t.Errorf("fee0 = %v, want %v", g0, tt.want0)
			}
			if math.Abs(g1-tt.want1) > 1e-9 {
				t.Errorf("fee1 = %v, want %v", g1, tt.want1)
			}
			if tt.c0.Cmp(c0Before) != 0 {
				t.Errorf("collect0 input was mutated: %v != %v", tt.c0, c0Before)
			}
		})
	}
}
