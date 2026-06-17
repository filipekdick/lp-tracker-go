package datasource

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
)

// Demo is a self-contained Source that synthesises realistic-looking pools and
// price histories. It lets the analyzer and dashboard run with zero external
// connectivity (useful for demos, offline development and CI). Each generated
// pool has a baked-in annualised volatility, and its price history is a
// geometric Brownian motion sampled hourly so realised volatility recovers it.
type Demo struct {
	rng *rand.Rand
}

// NewDemo builds a Demo source. A fixed seed keeps runs reproducible.
func NewDemo(seed int64) *Demo {
	return &Demo{rng: rand.New(rand.NewSource(seed))}
}

func (d *Demo) Name() string { return "demo" }

// dexByChain provides plausible DEX names per network.
var dexByChain = map[string][]string{
	"eth":         {"uniswap_v3", "uniswap_v4", "curve", "sushiswap"},
	"bsc":         {"pancakeswap_v3", "uniswap_v3", "thena"},
	"solana":      {"orca", "raydium_clmm", "meteora"},
	"avax":        {"trader_joe_v2", "pharaoh", "uniswap_v3"},
	"arbitrum":    {"uniswap_v3", "camelot_v3", "pancakeswap_v3"},
	"base":        {"aerodrome_slipstream", "uniswap_v3", "uniswap_v4"},
	"optimism":    {"velodrome_slipstream", "uniswap_v3"},
	"polygon_pos": {"uniswap_v3", "quickswap_v3"},
}

// assetTemplate describes a token and the kind of volatility regime it sits in.
type assetTemplate struct {
	symbol string
	price  float64
	vol    float64 // baseline annualised volatility
}

var volatileAssets = []assetTemplate{
	{"WETH", 3400, 0.55}, {"WBTC", 64000, 0.45}, {"SOL", 150, 0.85},
	{"ARB", 0.9, 1.05}, {"OP", 1.8, 1.0}, {"AVAX", 28, 0.95},
	{"LINK", 14, 0.8}, {"AERO", 1.1, 1.2}, {"PEPE", 0.000012, 1.6},
	{"WIF", 2.3, 1.7}, {"CRV", 0.32, 1.1}, {"LDO", 1.9, 1.0},
}

var stableAssets = []assetTemplate{
	{"USDC", 1, 0.02}, {"USDT", 1, 0.02}, {"DAI", 1, 0.03},
}

var feeTiers = []float64{0.0001, 0.0005, 0.003, 0.01}

// TopPools generates perChain pools for each chain.
func (d *Demo) TopPools(_ context.Context, chains []Chain, perChain int) ([]RawPool, error) {
	var out []RawPool
	for _, chain := range chains {
		dexes := dexByChain[chain.Slug]
		if len(dexes) == 0 {
			dexes = []string{"uniswap_v3"}
		}
		for i := 0; i < perChain; i++ {
			out = append(out, d.makePool(chain, dexes[i%len(dexes)]))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no chains configured")
	}
	return out, nil
}

func (d *Demo) makePool(chain Chain, dex string) RawPool {
	base := volatileAssets[d.rng.Intn(len(volatileAssets))]
	// ~70% of pools quote into a stablecoin, the rest are volatile/volatile.
	var quote assetTemplate
	if d.rng.Float64() < 0.7 {
		quote = stableAssets[d.rng.Intn(len(stableAssets))]
	} else {
		quote = volatileAssets[d.rng.Intn(len(volatileAssets))]
		for quote.symbol == base.symbol {
			quote = volatileAssets[d.rng.Intn(len(volatileAssets))]
		}
	}

	fee := feeTiers[d.rng.Intn(len(feeTiers))]
	// Effective volatility of the pair: stable quote ~= base vol; otherwise the
	// relative volatility of two correlated assets is a bit lower.
	vol := base.vol
	if quote.vol > 0.1 {
		vol = math.Hypot(base.vol, quote.vol) * 0.8
	}

	tvl := 250_000 + d.rng.Float64()*120_000_000
	// Daily volume / TVL turnover. Lower fee tiers attract proportionally more
	// flow, and we add a wide random spread so the scan yields a realistic mix
	// of attractive, fair and unattractive verdicts rather than all-green.
	turnover := (0.02 + d.rng.Float64()*0.6) * (0.003 / fee)
	volume := tvl * turnover

	closes := geometricBrownian(d.rng, base.price, vol, 168) // 7 days hourly
	price := closes[len(closes)-1]

	return RawPool{
		Chain:          chain.Display,
		ChainSlug:      chain.Slug,
		ChainKind:      chain.Kind,
		Address:        d.fakeAddress(chain.Slug),
		DEX:            dex,
		Protocol:       ProtocolFamily(dex),
		Name:           fmt.Sprintf("%s / %s", base.symbol, quote.symbol),
		BaseSymbol:     base.symbol,
		QuoteSymbol:    quote.symbol,
		FeeTier:        fee,
		TVLUSD:         tvl,
		Volume24hUSD:   volume,
		PriceUSD:       price,
		Closes:         closes,
		PeriodsPerYear: 24 * 365,
	}
}

func (d *Demo) fakeAddress(slug string) string {
	if slug == "solana" {
		const b58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
		var sb strings.Builder
		for i := 0; i < 44; i++ {
			sb.WriteByte(b58[d.rng.Intn(len(b58))])
		}
		return sb.String()
	}
	const hexd = "0123456789abcdef"
	var sb strings.Builder
	sb.WriteString("0x")
	for i := 0; i < 40; i++ {
		sb.WriteByte(hexd[d.rng.Intn(len(hexd))])
	}
	return sb.String()
}

// geometricBrownian returns n hourly close prices for a GBM with the given
// annualised volatility (and zero drift), starting from p0.
func geometricBrownian(rng *rand.Rand, p0, annualVol float64, n int) []float64 {
	const hoursPerYear = 24 * 365
	dt := 1.0 / hoursPerYear
	sigmaStep := annualVol * math.Sqrt(dt)

	closes := make([]float64, 0, n)
	p := p0
	for i := 0; i < n; i++ {
		shock := rng.NormFloat64() * sigmaStep
		p *= math.Exp(-0.5*sigmaStep*sigmaStep + shock)
		closes = append(closes, p)
	}
	return closes
}

// DemoImpliedVol is an offline ImpliedVolSource for BTC/ETH-like assets.
type DemoImpliedVol struct{}

// ImpliedVol returns a plausible options-implied vol for major assets.
func (DemoImpliedVol) ImpliedVol(_ context.Context, symbol string) (float64, bool) {
	switch NormalizeSymbol(symbol) {
	case "BTC":
		return 0.48, true
	case "ETH":
		return 0.60, true
	default:
		return 0, false
	}
}
