package onchain

import (
	"context"
	"testing"
)

func TestSupportedDEX(t *testing.T) {
	supported := []string{
		"uniswap_v3", "uniswap-v3", "pancakeswap_v3",
		"aerodrome_slipstream", "aerodrome-slipstream", "aerodrome_cl",
		"velodrome_cl",
	}
	for _, d := range supported {
		if !SupportedDEX(d) {
			t.Errorf("expected %q supported", d)
		}
	}
	unsupported := []string{
		"", "uniswap_v2", "uniswap_v4", "camelot_v3", "algebra",
		"thena_fusion", "quickswap_v3", "curve", "aerodrome", // plain v2 aerodrome
	}
	for _, d := range unsupported {
		if SupportedDEX(d) {
			t.Errorf("expected %q unsupported", d)
		}
	}
}

func TestNoopProberAlwaysUnsupported(t *testing.T) {
	st, ok, err := NoopProber{}.PoolStateAt(context.Background(), "base", "0xpool")
	if ok || err != nil || st.SqrtPriceX96 != nil {
		t.Fatalf("noop prober should report unsupported: ok=%v err=%v", ok, err)
	}
}
