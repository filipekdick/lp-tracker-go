package analyzer

import (
	"math"
	"math/rand"
	"testing"
)

// closeBar makes a degenerate OHLCV bar (O=H=L=C=p) for close-to-close tests.
func closeBar(p float64) OHLCV {
	return OHLCV{Open: p, High: p, Low: p, Close: p, Volume: 1}
}

// TestRatioVolLowerThanUSD verifies the Part A correction: for two positively
// correlated assets (a shared market factor), the realized volatility of the
// pool RATIO (base priced in quote) is strictly lower than the base asset's
// USD volatility, because the common factor cancels in the ratio.
//
// Construction: per-bar log returns
//
//	base_ret  = dM + db        quote_ret = dM + dq        ratio_ret = base_ret - quote_ret = db - dq
//
// so vol(base) = sqrt(σM² + σb²) while vol(ratio) = sqrt(σb² + σq²). With a large
// common factor σM the ratio vol is much smaller. Fixed seed => reproducible.
func TestRatioVolLowerThanUSD(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const (
		n   = 4000
		ppy = hoursPerYear
	)
	perBar := func(annual float64) float64 { return annual * math.Sqrt(1.0/ppy) }
	sM, sb, sq := perBar(0.70), perBar(0.30), perBar(0.30)

	lb, lq := math.Log(3000.0), math.Log(60000.0)
	lr := lb - lq
	baseUSD := make([]OHLCV, 0, n)
	ratio := make([]OHLCV, 0, n)
	for i := 0; i < n; i++ {
		dM := rng.NormFloat64() * sM
		db := dM + rng.NormFloat64()*sb
		dq := dM + rng.NormFloat64()*sq
		lb += db
		lr += db - dq
		baseUSD = append(baseUSD, closeBar(math.Exp(lb)))
		ratio = append(ratio, closeBar(math.Exp(lr)))
	}

	vBase, ok1 := CloseToCloseVol(baseUSD, ppy)
	vRatio, ok2 := CloseToCloseVol(ratio, ppy)
	if !ok1 || !ok2 {
		t.Fatal("vol estimators returned not-ok on clean series")
	}
	if !(vRatio < vBase) {
		t.Fatalf("ratio vol %.4f should be strictly below base-USD vol %.4f", vRatio, vBase)
	}
	// Sanity: vol(base) ≈ sqrt(0.7²+0.3²)=0.762, vol(ratio) ≈ sqrt(0.3²+0.3²)=0.424.
	if math.Abs(vBase-0.762) > 0.05 {
		t.Fatalf("base vol %.4f, want ≈0.762", vBase)
	}
	if math.Abs(vRatio-0.424) > 0.05 {
		t.Fatalf("ratio vol %.4f, want ≈0.424", vRatio)
	}
}

// TestRatioVolMatchesUSDForStableQuote is the regression guard for token/stable
// pairs: when the quote is a stablecoin (flat, no market-factor exposure), the
// ratio price equals the base USD price, so the two volatilities match tightly.
func TestRatioVolMatchesUSDForStableQuote(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	const (
		n   = 4000
		ppy = hoursPerYear
	)
	perBar := func(annual float64) float64 { return annual * math.Sqrt(1.0/ppy) }
	sM, sb := perBar(0.70), perBar(0.30)

	lb := math.Log(3000.0)
	lq := math.Log(1.0) // stablecoin: flat
	lr := lb - lq
	baseUSD := make([]OHLCV, 0, n)
	ratio := make([]OHLCV, 0, n)
	for i := 0; i < n; i++ {
		db := rng.NormFloat64()*sM + rng.NormFloat64()*sb
		// Stablecoin quote: no return (dq = 0), so ratio_ret == base_ret.
		lb += db
		lr += db
		baseUSD = append(baseUSD, closeBar(math.Exp(lb)))
		ratio = append(ratio, closeBar(math.Exp(lr)))
	}
	vBase, _ := CloseToCloseVol(baseUSD, ppy)
	vRatio, _ := CloseToCloseVol(ratio, ppy)
	// Identical construction => identical to floating-point tolerance.
	if math.Abs(vBase-vRatio) > 1e-9 {
		t.Fatalf("stable-quote ratio vol %.6f should match base-USD vol %.6f", vRatio, vBase)
	}
}
