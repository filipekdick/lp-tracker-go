package analyzer

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/filipekdick/lp-tracker-go/internal/onchain"
	"github.com/filipekdick/lp-tracker-go/internal/v3math"
)

// fakeProber returns a fixed PoolState, so the on-chain path is exercised with no
// network.
type fakeProber struct {
	st onchain.PoolState
	ok bool
}

func (f fakeProber) PoolStateAt(context.Context, string, string) (onchain.PoolState, bool, error) {
	return f.st, f.ok, nil
}

// ethUSDCState builds a plausible WETH(18)/USDC(6) pool state at ~$3000 with a
// known active liquidity, token0 = WETH.
func ethUSDCState(liquidity *big.Int) onchain.PoolState {
	// price ~3000 USDC per WETH. tick for that price:
	tick := v3math.TickAtPrice(3000, 18, 6)
	sqrt := v3math.GetSqrtRatioAtTick(tick)
	return onchain.PoolState{
		SqrtPriceX96: sqrt,
		TickNow:      tick,
		TickSpacing:  10,
		Liquidity:    liquidity,
		Dec0:         18,
		Dec1:         6,
	}
}

func TestConcentratedProjectionOnchainShareModel(t *testing.T) {
	st := ethUSDCState(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)) // 1e20 raw L
	in := ConcentratedInput{
		ChainSlug:    "base",
		PoolAddress:  "0xpool",
		DEX:          "uniswap_v3",
		FeeTier:      0.0005,
		TVLUSD:       5_000_000,
		Volume24hUSD: 3_000_000,
		RealizedVol:  0.6,
		BaseUSD:      3000, // WETH
		QuoteUSD:     1,    // USDC
	}
	sim := ConcentratedProjection(context.Background(), fakeProber{st: st, ok: true}, in, true)
	if sim == nil {
		t.Fatal("expected a sim from the on-chain path")
	}
	if sim.Source != "onchain" {
		t.Fatalf("source = %q, want onchain", sim.Source)
	}
	b1, ok1 := sim.Bands["1"]
	b2, ok2 := sim.Bands["2"]
	if !ok1 || !ok2 {
		t.Fatalf("expected bands 1 and 2, got %+v", sim.Bands)
	}
	// Share must be bounded — no blow-up.
	if b1.ShareOfLiquidity <= 0 || b1.ShareOfLiquidity > 1 {
		t.Fatalf("share out of bounds: %v", b1.ShareOfLiquidity)
	}
	// APR must equal annualFees * share / deposit for the band (hand identity).
	annualFees := in.Volume24hUSD * in.FeeTier * 365.0
	wantAPR := annualFees * b1.ShareOfLiquidity / ConcDepositUSD
	if math.Abs(b1.APR-wantAPR) > 1e-9 {
		t.Fatalf("on-chain APR = %v, want %v", b1.APR, wantAPR)
	}
	// Tighter band (k=1) earns more than wider band (k=2), both bounded.
	if !(b1.APR > b2.APR) {
		t.Fatalf("k=1 APR %v should exceed k=2 APR %v", b1.APR, b2.APR)
	}
	if b1.APR <= 0 || math.IsInf(b1.APR, 0) || math.IsNaN(b1.APR) {
		t.Fatalf("k=1 APR not finite/positive: %v", b1.APR)
	}
}

func TestConcentratedProjectionFallbackEqualsFeeAPRAtK1(t *testing.T) {
	in := ConcentratedInput{
		ChainSlug:    "base",
		PoolAddress:  "0xpool",
		DEX:          "uniswap_v3",
		FeeTier:      0.0005,
		TVLUSD:       5_000_000,
		Volume24hUSD: 3_000_000,
		RealizedVol:  0.6,
		BaseUSD:      3000,
		QuoteUSD:     1,
	}
	// NoopProber forces the estimated fallback.
	sim := ConcentratedProjection(context.Background(), onchain.NoopProber{}, in, true)
	if sim == nil {
		t.Fatal("expected an estimated sim")
	}
	if sim.Source != "estimated" {
		t.Fatalf("source = %q, want estimated", sim.Source)
	}
	b1 := sim.Bands["1"]
	// At k=1 the fallback APR equals the pool full-range fee APR (modulo the tiny
	// deposit dilution).
	poolAPR := FeeAPR(in.Volume24hUSD, in.FeeTier, in.TVLUSD)
	if rel := math.Abs(b1.APR-poolAPR) / poolAPR; rel > 0.01 {
		t.Fatalf("fallback k=1 APR = %v, want ~poolFeeAPR %v (rel %v)", b1.APR, poolAPR, rel)
	}
	// k=1 still beats k=2 in the fallback too.
	if !(b1.APR > sim.Bands["2"].APR) {
		t.Fatalf("fallback k=1 %v should exceed k=2 %v", b1.APR, sim.Bands["2"].APR)
	}
}

func TestConcentratedProjectionNoSignal(t *testing.T) {
	// No volatility signal -> nil (never errors the scan).
	in := ConcentratedInput{
		DEX: "uniswap_v3", FeeTier: 0.0005, TVLUSD: 1e6, Volume24hUSD: 1e6, RealizedVol: 0,
		BaseUSD: 3000, QuoteUSD: 1,
	}
	if sim := ConcentratedProjection(context.Background(), onchain.NoopProber{}, in, true); sim != nil {
		t.Fatalf("expected nil with no vol signal, got %+v", sim)
	}
}

func TestConcentratedProjectionUnsupportedDEXFallsBack(t *testing.T) {
	// A supported-looking probe but an Algebra DEX: must use the estimate, never
	// the prober (which here would claim ok).
	in := ConcentratedInput{
		ChainSlug: "arbitrum", PoolAddress: "0xpool", DEX: "camelot_v3",
		FeeTier: 0.0005, TVLUSD: 5e6, Volume24hUSD: 3e6, RealizedVol: 0.6,
		BaseUSD: 3000, QuoteUSD: 1,
	}
	st := ethUSDCState(big.NewInt(1))
	sim := ConcentratedProjection(context.Background(), fakeProber{st: st, ok: true}, in, true)
	if sim == nil || sim.Source != "estimated" {
		t.Fatalf("camelot_v3 must fall back to estimated, got %+v", sim)
	}
}
