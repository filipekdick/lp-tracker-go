package position

import (
	"context"
	"testing"
)

func TestHedgeLegPicksVolatileLeg(t *testing.T) {
	// WETH/USDC -> hedge WETH on ETHUSDT.
	asset, amt, perp, ok := hedgeLeg("WETH", "USDC", 4.2, 14000)
	if !ok || asset != "WETH" || perp != "ETHUSDT" || amt != 4.2 {
		t.Fatalf("got asset=%q amt=%v perp=%q ok=%v", asset, amt, perp, ok)
	}

	// Order shouldn't matter: USDC/WETH still hedges the WETH leg.
	asset, amt, perp, ok = hedgeLeg("USDC", "WETH", 14000, 4.2)
	if !ok || asset != "WETH" || perp != "ETHUSDT" || amt != 4.2 {
		t.Fatalf("reversed: got asset=%q amt=%v perp=%q ok=%v", asset, amt, perp, ok)
	}
}

func TestHedgeLegsMultiLeg(t *testing.T) {
	// WETH/VVV -> should return both legs.
	legs := hedgeLegs("WETH", "VVV", 2.1, 268.29)
	if len(legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(legs))
	}
	if legs[0].Asset != "WETH" || legs[0].Perp != "ETHUSDT" || legs[0].Amount != 2.1 {
		t.Errorf("leg 0 mismatch: %+v", legs[0])
	}
	if legs[1].Asset != "VVV" || legs[1].Perp != "VVVUSDT" || legs[1].Amount != 268.29 {
		t.Errorf("leg 1 mismatch: %+v", legs[1])
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
	if tp.Hedges[0].Symbol != "ETHUSDT" || tp.Hedges[0].ExposureSymbol != "WETH" {
		t.Fatalf("unexpected hedge: %+v", tp.Hedges)
	}
	// Target short should equal the WETH leg held in the LP.
	if tp.Hedges[0].TargetShort != tp.Amount0 {
		t.Fatalf("target short %v should match WETH amount %v", tp.Hedges[0].TargetShort, tp.Amount0)
	}
	// The analysis should have run and produced a fee-implied volatility.
	if tp.Analysis.FeeImpliedVol <= 0 {
		t.Fatalf("expected positive fee-implied vol, got %v", tp.Analysis.FeeImpliedVol)
	}
	if !tp.Analysis.HasImplied {
		t.Fatal("expected Deribit/demo implied vol for an ETH pool")
	}
}
