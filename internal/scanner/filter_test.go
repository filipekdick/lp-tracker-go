package scanner

import (
	"testing"

	"github.com/filipekdick/lp-tracker-go/internal/datasource"
)

// TestFilterPools checks the junk-pool filter: dust pools (below the TVL floor)
// and wash-trade pools (turnover above the cap) are dropped, while a legitimate
// large pool with both high volume and high TVL is kept.
func TestFilterPools(t *testing.T) {
	const (
		minTVL      = 250_000.0
		maxTurnover = 20.0
	)
	pools := []datasource.RawPool{
		{Name: "legit", TVLUSD: 10_000_000, Volume24hUSD: 5_000_000}, // turnover 0.5, kept
		{Name: "dust", TVLUSD: 50_000, Volume24hUSD: 100_000},        // below TVL floor
		{Name: "wash", TVLUSD: 100_000, Volume24hUSD: 50_000_000},    // turnover 500, dropped (also dust)
		{Name: "washBigTVL", TVLUSD: 1_000_000, Volume24hUSD: 5e7},   // turnover 50, dropped on cap alone
		{Name: "edgeTVL", TVLUSD: 250_000, Volume24hUSD: 1_000_000},  // exactly floor, turnover 4, kept
	}
	got := filterPools(pools, minTVL, maxTurnover)

	kept := map[string]bool{}
	for _, p := range got {
		kept[p.Name] = true
	}
	if !kept["legit"] {
		t.Error("legitimate large pool was dropped")
	}
	if !kept["edgeTVL"] {
		t.Error("pool exactly at the TVL floor should be kept")
	}
	if kept["dust"] {
		t.Error("dust pool below TVL floor should be dropped")
	}
	if kept["wash"] {
		t.Error("wash-trade pool should be dropped")
	}
	if kept["washBigTVL"] {
		t.Error("high-turnover pool should be dropped on the turnover cap alone")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 surviving pools, got %d", len(got))
	}
}

// TestFilterPoolsDisabled checks that non-positive thresholds disable the checks.
func TestFilterPoolsDisabled(t *testing.T) {
	pools := []datasource.RawPool{
		{Name: "a", TVLUSD: 1, Volume24hUSD: 1_000_000},
		{Name: "b", TVLUSD: 0, Volume24hUSD: 0},
	}
	got := filterPools(pools, 0, 0)
	if len(got) != 2 {
		t.Fatalf("disabled filter should keep all pools, got %d", len(got))
	}
}
