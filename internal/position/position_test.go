package position

import (
	"context"
	"testing"
)

func TestHedgeLegPicksVolatileLeg(t *testing.T) {
	// WETH/USDC -> hedge WETH on ETHUSDC (USDC-preferred default).
	asset, amt, perp, ok := hedgeLeg("WETH", "USDC", 4.2, 14000)
	if !ok || asset != "WETH" || perp != "ETHUSDC" || amt != 4.2 {
		t.Fatalf("got asset=%q amt=%v perp=%q ok=%v", asset, amt, perp, ok)
	}

	// Order shouldn't matter: USDC/WETH still hedges the WETH leg.
	asset, amt, perp, ok = hedgeLeg("USDC", "WETH", 14000, 4.2)
	if !ok || asset != "WETH" || perp != "ETHUSDC" || amt != 4.2 {
		t.Fatalf("reversed: got asset=%q amt=%v perp=%q ok=%v", asset, amt, perp, ok)
	}
}

func TestHedgeLegsMultiLeg(t *testing.T) {
	// WETH/VVV -> should return both legs. ETH has a USDC perp (ETHUSDC); VVV does
	// not, so it falls back to VVVUSDT.
	legs := hedgeLegs("WETH", "VVV", 2.1, 268.29)
	if len(legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(legs))
	}
	if legs[0].Asset != "WETH" || legs[0].Perp != "ETHUSDC" || legs[0].Amount != 2.1 {
		t.Errorf("leg 0 mismatch: %+v", legs[0])
	}
	if legs[1].Asset != "VVV" || legs[1].Perp != "VVVUSDT" || legs[1].Amount != 268.29 {
		t.Errorf("leg 1 mismatch: %+v", legs[1])
	}
}

func TestResolvePerpPrefersUSDC(t *testing.T) {
	defer func() { HedgeQuotePreference = "usdc" }()

	HedgeQuotePreference = "usdc"
	if got, ok := resolvePerp("ETH"); !ok || got != "ETHUSDC" {
		t.Fatalf("ETH usdc: got %q ok=%v", got, ok)
	}
	// VVV has no USDC perp listed -> falls back to USDT even under usdc preference.
	if got, ok := resolvePerp("VVV"); !ok || got != "VVVUSDT" {
		t.Fatalf("VVV fallback: got %q ok=%v", got, ok)
	}
	// Forcing usdt overrides the preference for assets that do have a USDC perp.
	HedgeQuotePreference = "usdt"
	if got, ok := resolvePerp("ETH"); !ok || got != "ETHUSDT" {
		t.Fatalf("ETH forced usdt: got %q ok=%v", got, ok)
	}
	// Unknown asset is not hedgeable.
	if _, ok := resolvePerp("FOO"); ok {
		t.Fatal("FOO should have no perp")
	}
}

func TestAggregateExposuresFoldsSynthetics(t *testing.T) {
	legs := []PositionLeg{
		{TokenID: 1, Symbol0: "WETH", Symbol1: "USDC", Amount0: 4, Amount1: 12000},
		{TokenID: 2, Symbol0: "wstETH", Symbol1: "WETH", Amount0: 1.5, Amount1: 0.5},
		{TokenID: 3, Symbol0: "cbBTC", Symbol1: "USDC", Amount0: 0.2, Amount1: 9000},
	}
	ex := aggregateExposures(legs)
	byAsset := map[string]AssetExposure{}
	for _, e := range ex {
		byAsset[e.Asset] = e
	}
	eth, ok := byAsset["ETH"]
	if !ok {
		t.Fatal("expected aggregated ETH exposure")
	}
	// 4 (WETH) + 1.5 (wstETH) + 0.5 (WETH) all fold into ETH.
	if eth.Amount != 6.0 {
		t.Fatalf("ETH amount: got %v want 6.0", eth.Amount)
	}
	if len(eth.Sources) != 3 {
		t.Fatalf("ETH sources: got %d want 3", len(eth.Sources))
	}
	if eth.Perp != "ETHUSDC" {
		t.Fatalf("ETH perp: got %q want ETHUSDC", eth.Perp)
	}
	btc, ok := byAsset["BTC"]
	if !ok || btc.Amount != 0.2 || btc.Perp != "BTCUSDC" {
		t.Fatalf("BTC exposure wrong: %+v", btc)
	}
	// Stablecoins must never produce an exposure.
	if _, has := byAsset["USDC"]; has {
		t.Fatal("USDC should not be an exposure")
	}
	// Largest exposure (ETH) must lead.
	if ex[0].Asset != "ETH" {
		t.Fatalf("expected ETH first, got %q", ex[0].Asset)
	}
}

func TestHedgeLegUnhedgeablePool(t *testing.T) {
	// Two stablecoins -> nothing to hedge.
	if _, _, _, ok := hedgeLeg("USDC", "DAI", 1, 1); ok {
		t.Fatal("stable/stable pool should not be hedgeable")
	}
	// Unknown alt with no perp listing -> not hedgeable.
	if _, _, _, ok := hedgeLeg("FOO", "USDC", 1, 1); ok {
		t.Fatal("alt without a perp should not be hedgeable")
	}
}

func TestDemoTrackerIsConsistent(t *testing.T) {
	tp, err := NewDemoTracker(71002035).Track(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tp.Protocol != "Aerodrome" || tp.ChainSlug != "base" {
		t.Fatalf("expected Aerodrome on base, got %s on %s", tp.Protocol, tp.ChainSlug)
	}
	if tp.Hedges[0].Symbol != "ETHUSDC" || tp.Hedges[0].ExposureSymbol != "WETH" {
		t.Fatalf("unexpected hedge: %+v", tp.Hedges)
	}
	// The demo aggregates ETH exposure across two positions, so the single short's
	// target must equal the summed ETH units, not just the primary WETH leg.
	if len(tp.Exposures) == 0 || tp.Exposures[0].Asset != "ETH" {
		t.Fatalf("expected aggregated ETH exposure, got %+v", tp.Exposures)
	}
	if tp.Hedges[0].TargetShort != tp.Exposures[0].Amount {
		t.Fatalf("target short %v should match aggregated ETH %v", tp.Hedges[0].TargetShort, tp.Exposures[0].Amount)
	}
	// The analysis should have run and produced a fee-implied volatility.
	if tp.Analysis.FeeImpliedVol <= 0 {
		t.Fatalf("expected positive fee-implied vol, got %v", tp.Analysis.FeeImpliedVol)
	}
	if !tp.Analysis.HasImplied {
		t.Fatal("expected Deribit/demo implied vol for an ETH pool")
	}
}
