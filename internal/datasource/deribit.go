package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// deribitBaseURL is the Deribit public API root. Override via DERIBIT_URL.
const deribitBaseURL = "https://www.deribit.com/api/v2"

// Deribit supplies options-implied volatility for the assets that have a liquid
// options market there (currently BTC and ETH via the DVOL index). Symbols are
// normalised so wrapped/bridged variants (WETH, WBTC, cbBTC, ...) map to their
// underlying currency. Results are cached briefly to avoid hammering the API.
type Deribit struct {
	baseURL string
	http    *http.Client

	mu    sync.Mutex
	cache map[string]cachedVol
	ttl   time.Duration
}

type cachedVol struct {
	vol float64
	ok  bool
	at  time.Time
}

// NewDeribit builds a Deribit implied-vol source. baseURL may be empty.
func NewDeribit(baseURL string) *Deribit {
	if baseURL == "" {
		baseURL = deribitBaseURL
	}
	return &Deribit{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
		cache:   make(map[string]cachedVol),
		ttl:     5 * time.Minute,
	}
}

// supported maps a normalised currency to its Deribit DVOL currency code.
var deribitCurrencies = map[string]string{
	"BTC": "BTC",
	"ETH": "ETH",
}

// normalizeSymbol strips common wrapper/bridge prefixes so e.g. WETH -> ETH.
func normalizeSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	switch s {
	case "WETH", "WEETH", "WSTETH", "STETH", "RETH", "CBETH":
		return "ETH"
	case "WBTC", "CBBTC", "TBTC", "BTCB", "RENBTC":
		return "BTC"
	}
	return s
}

// ImpliedVol returns the annualised options-implied volatility for an asset, or
// ok=false when none is available.
func (d *Deribit) ImpliedVol(ctx context.Context, symbol string) (float64, bool) {
	cur, supported := deribitCurrencies[normalizeSymbol(symbol)]
	if !supported {
		return 0, false
	}

	d.mu.Lock()
	if c, ok := d.cache[cur]; ok && time.Since(c.at) < d.ttl {
		d.mu.Unlock()
		return c.vol, c.ok
	}
	d.mu.Unlock()

	vol, ok := d.fetchDVOL(ctx, cur)

	d.mu.Lock()
	d.cache[cur] = cachedVol{vol: vol, ok: ok, at: time.Now()}
	d.mu.Unlock()
	return vol, ok
}

// fetchDVOL reads the latest DVOL index value. Deribit reports DVOL in percent
// (e.g. 55 == 55% annualised), which we convert to a fraction.
func (d *Deribit) fetchDVOL(ctx context.Context, currency string) (float64, bool) {
	now := time.Now().UnixMilli()
	start := now - int64(24*time.Hour/time.Millisecond)
	url := fmt.Sprintf("%s/public/get_volatility_index_data?currency=%s&start_timestamp=%d&end_timestamp=%d&resolution=3600",
		d.baseURL, currency, start, now)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}
	res, err := d.http.Do(req)
	if err != nil {
		return 0, false
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil || res.StatusCode != http.StatusOK {
		return 0, false
	}

	var parsed struct {
		Result struct {
			// Each row: [timestamp, open, high, low, close]
			Data [][]float64 `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, false
	}
	rows := parsed.Result.Data
	if len(rows) == 0 {
		return 0, false
	}
	last := rows[len(rows)-1]
	if len(last) < 5 || last[4] <= 0 {
		return 0, false
	}
	return last[4] / 100.0, true
}
