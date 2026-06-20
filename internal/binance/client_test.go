package binance

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func decimals(s string) int {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

func TestMakerLimitPrice(t *testing.T) {
	cases := []struct {
		name      string
		mark      float64
		tick      float64
		precision int
		side      Side
	}{
		{"ETHUSDT sell", 1733.04, 0.01, 2, SideSell},
		{"ETHUSDT buy", 1733.04, 0.01, 2, SideBuy},
		{"BTCUSDT sell", 64812.3, 0.1, 1, SideSell},
		{"SOLUSDT buy", 153.647, 0.01, 2, SideBuy},
		{"low price sell", 0.5823, 0.0001, 4, SideSell},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := makerLimitPrice(c.mark, c.tick, c.precision, c.side)
			p, err := strconv.ParseFloat(got, 64)
			if err != nil {
				t.Fatalf("unparseable price %q: %v", got, err)
			}
			// -1111 guard: never more decimals than the symbol allows.
			if d := decimals(got); d > c.precision {
				t.Errorf("price %q has %d decimals, want <= %d", got, d, c.precision)
			}
			// Must be a whole multiple of the tick.
			if rem := math.Abs(math.Round(p/c.tick)*c.tick - p); rem > c.tick/2 {
				t.Errorf("price %v is not aligned to tick %v", p, c.tick)
			}
			// -5022 guard: a maker sell rests above the mark, a maker buy below.
			if c.side == SideSell && p <= c.mark {
				t.Errorf("sell maker price %v must be above mark %v", p, c.mark)
			}
			if c.side == SideBuy && p >= c.mark {
				t.Errorf("buy maker price %v must be below mark %v", p, c.mark)
			}
		})
	}
}

func TestRoundToTick(t *testing.T) {
	if got := roundToTick(100.077, 0.01, true); math.Abs(got-100.08) > 1e-9 {
		t.Errorf("ceil: got %v want 100.08", got)
	}
	if got := roundToTick(100.077, 0.01, false); math.Abs(got-100.07) > 1e-9 {
		t.Errorf("floor: got %v want 100.07", got)
	}
	// Zero tick is a no-op rather than a divide-by-zero.
	if got := roundToTick(42.5, 0, true); got != 42.5 {
		t.Errorf("zero tick: got %v want 42.5", got)
	}
}
