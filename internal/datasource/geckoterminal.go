package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// geckoBaseURL is the public GeckoTerminal API v2 root. Override via the
// GECKOTERMINAL_URL env var if proxying.
const geckoBaseURL = "https://api.coingecko.com/api/v3/onchain"

// GeckoTerminal is a live Source backed by the GeckoTerminal public API. It
// lists the most active pools per network and pulls hourly OHLCV to measure
// realised volatility.
type GeckoTerminal struct {
	baseURL string
	apiKey  string
	http    *http.Client

	mu sync.Mutex
	// reqGap throttles requests to stay within the public rate limit
	// (~30 req/min). It is applied before every HTTP call.
	reqGap   time.Duration
	lastCall time.Time
}

// NewGeckoTerminal builds a GeckoTerminal source. baseURL may be empty to use
// the public CoinGecko onchain endpoint. When an apiKey is supplied it is sent
// as a CoinGecko demo key, which lifts the rate limit to ~30 req/min, so we can
// poll roughly 3x faster than the unauthenticated ~9 req/min.
func NewGeckoTerminal(baseURL, apiKey string) *GeckoTerminal {
	if baseURL == "" {
		baseURL = geckoBaseURL
	}
	reqGap := 6500 * time.Millisecond // ~9 req/min, safe without a key
	if apiKey != "" {
		reqGap = 2100 * time.Millisecond // ~28 req/min, under the demo cap
	}
	return &GeckoTerminal{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
		reqGap:  reqGap,
	}
}

func (g *GeckoTerminal) Name() string { return "geckoterminal" }

// TopPools fetches the most active pools (by 24h volume) for each chain and
// enriches the top results with hourly price history.
func (g *GeckoTerminal) TopPools(ctx context.Context, chains []Chain, perChain int) ([]RawPool, error) {
	var out []RawPool
	for _, chain := range chains {
		pools, err := g.poolsForChain(ctx, chain, perChain)
		if err != nil {
			// One failing chain should not abort the whole scan.
			fmt.Printf("[geckoterminal] %s: %v\n", chain.Slug, err)
			continue
		}
		out = append(out, pools...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no pools returned from any chain")
	}
	return out, nil
}

func (g *GeckoTerminal) poolsForChain(ctx context.Context, chain Chain, perChain int) ([]RawPool, error) {
	var resp poolsResponse
	url := fmt.Sprintf("%s/networks/%s/pools?page=1&sort=h24_volume_usd_desc", g.baseURL, chain.Slug)
	if err := g.getJSON(ctx, url, &resp); err != nil {
		return nil, err
	}

	symbols := indexTokenSymbols(resp.Included)

	pools := make([]RawPool, 0, perChain)
	for _, d := range resp.Data {
		if len(pools) >= perChain {
			break
		}
		rp, ok := g.toRawPool(ctx, chain, d, symbols)
		if !ok {
			continue
		}
		pools = append(pools, rp)
	}
	return pools, nil
}

// PoolByAddress fetches a single pool's market data (plus hourly OHLCV) for the
// given chain. It is used to analyse a specifically tracked LP position.
func (g *GeckoTerminal) PoolByAddress(ctx context.Context, chain Chain, address string) (RawPool, error) {
	var resp singlePoolResponse
	url := fmt.Sprintf("%s/networks/%s/pools/%s", g.baseURL, chain.Slug, address)
	if err := g.getJSON(ctx, url, &resp); err != nil {
		return RawPool{}, err
	}
	symbols := indexTokenSymbols(resp.Included)
	rp, ok := g.toRawPool(ctx, chain, resp.Data, symbols)
	if !ok {
		return RawPool{}, fmt.Errorf("pool %s on %s returned no usable data", address, chain.Slug)
	}
	return rp, nil
}

// toRawPool turns one API pool object into a RawPool, enriching it with hourly
// price history. ok is false when the pool lacks usable TVL/volume.
func (g *GeckoTerminal) toRawPool(ctx context.Context, chain Chain, d poolData, symbols map[string]string) (RawPool, bool) {
	a := d.Attributes
	tvl := parseFloat(a.ReserveInUSD)
	vol := parseFloat(a.VolumeUSD.H24)
	if tvl <= 0 {
		return RawPool{}, false
	}

	base, quote := splitPairName(a.Name)
	if s, ok := symbols[d.Relationships.BaseToken.Data.ID]; ok {
		base = s
	}
	if s, ok := symbols[d.Relationships.QuoteToken.Data.ID]; ok {
		quote = s
	}

	closes, ppy := g.priceHistory(ctx, chain.Slug, a.Address)

	return RawPool{
		Chain:          chain.Display,
		ChainSlug:      chain.Slug,
		ChainKind:      chain.Kind,
		Address:        a.Address,
		DEX:            d.Relationships.DEX.Data.ID,
		Protocol:       ProtocolFamily(d.Relationships.DEX.Data.ID),
		Name:           fmt.Sprintf("%s / %s", base, quote),
		BaseSymbol:     base,
		QuoteSymbol:    quote,
		FeeTier:        feeTier(a),
		TVLUSD:         tvl,
		Volume24hUSD:   vol,
		PriceUSD:       parseFloat(a.BaseTokenPriceUSD),
		QuotePriceUSD:  parseFloat(a.QuoteTokenPriceUSD),
		Closes:         closes,
		PeriodsPerYear: ppy,
	}, true
}

// priceHistory returns up to 7 days of hourly closes (oldest first) and the
// matching periods-per-year. On failure it returns nil closes so the analyzer
// simply skips realised volatility for that pool.
func (g *GeckoTerminal) priceHistory(ctx context.Context, slug, address string) ([]float64, float64) {
	var resp ohlcvResponse
	url := fmt.Sprintf("%s/networks/%s/pools/%s/ohlcv/hour?aggregate=1&limit=168&currency=usd", g.baseURL, slug, address)
	if err := g.getJSON(ctx, url, &resp); err != nil {
		return nil, 0
	}
	rows := resp.Data.Attributes.OHLCVList
	if len(rows) < 3 {
		return nil, 0
	}
	// Each row is [timestamp, open, high, low, close, volume]; sort ascending.
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	closes := make([]float64, 0, len(rows))
	for _, r := range rows {
		if len(r) >= 5 && r[4] > 0 {
			closes = append(closes, r[4])
		}
	}
	const hoursPerYear = 24 * 365
	return closes, hoursPerYear
}

func (g *GeckoTerminal) getJSON(ctx context.Context, url string, dst any) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Throttle to respect the public rate limit.
	if wait := g.reqGap - time.Since(g.lastCall); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	g.lastCall = time.Now()

	reqCtx, cancel := context.WithTimeout(context.Background(), g.http.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json;version=20230302")
	req.Header.Set("User-Agent", "lp-tracker-go/analyzer")
	if g.apiKey != "" {
		req.Header.Set("x-cg-demo-api-key", g.apiKey)
	}

	res, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", res.StatusCode, snippet(body))
	}
	return json.Unmarshal(body, dst)
}

// --- response shapes -------------------------------------------------------

type poolsResponse struct {
	Data     []poolData     `json:"data"`
	Included []includedData `json:"included"`
}

type singlePoolResponse struct {
	Data     poolData       `json:"data"`
	Included []includedData `json:"included"`
}

type poolData struct {
	ID            string         `json:"id"`
	Attributes    poolAttributes `json:"attributes"`
	Relationships struct {
		BaseToken  relRef `json:"base_token"`
		QuoteToken relRef `json:"quote_token"`
		DEX        relRef `json:"dex"`
	} `json:"relationships"`
}

type poolAttributes struct {
	Name               string `json:"name"`
	Address            string `json:"address"`
	BaseTokenPriceUSD  string `json:"base_token_price_usd"`
	QuoteTokenPriceUSD string `json:"quote_token_price_usd"`
	ReserveInUSD       string `json:"reserve_in_usd"`
	PoolFeePercentage  string `json:"pool_fee_percentage"`
	VolumeUSD          struct {
		H24 string `json:"h24"`
	} `json:"volume_usd"`
}

type relRef struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

type includedData struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Symbol string `json:"symbol"`
	} `json:"attributes"`
}

type ohlcvResponse struct {
	Data struct {
		Attributes struct {
			OHLCVList [][]float64 `json:"ohlcv_list"`
		} `json:"attributes"`
	} `json:"data"`
}

// --- helpers ---------------------------------------------------------------

func indexTokenSymbols(included []includedData) map[string]string {
	m := make(map[string]string, len(included))
	for _, inc := range included {
		if inc.Type == "token" && inc.Attributes.Symbol != "" {
			m[inc.ID] = inc.Attributes.Symbol
		}
	}
	return m
}

// feeTier resolves a fractional fee from the explicit field when present, else
// from the pool name suffix (e.g. "... 0.05%"), defaulting to 0.3%.
func feeTier(a poolAttributes) float64 {
	if pct := parseFloat(a.PoolFeePercentage); pct > 0 {
		return pct / 100.0
	}
	if pct := feeFromName(a.Name); pct > 0 {
		return pct / 100.0
	}
	return 0.003
}

func feeFromName(name string) float64 {
	i := strings.LastIndex(name, "%")
	if i <= 0 {
		return 0
	}
	// Walk back over the number preceding the percent sign.
	j := i - 1
	for j >= 0 && (name[j] == '.' || (name[j] >= '0' && name[j] <= '9')) {
		j--
	}
	return parseFloat(name[j+1 : i])
}

func splitPairName(name string) (base, quote string) {
	// Names look like "WETH / USDC 0.05%".
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return name, ""
	}
	base = strings.TrimSpace(parts[0])
	quote = strings.TrimSpace(parts[1])
	// Strip a trailing fee token from the quote (e.g. "USDC 0.05%").
	if fields := strings.Fields(quote); len(fields) > 0 {
		quote = fields[0]
	}
	return base, quote
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max])
	}
	return string(b)
}
