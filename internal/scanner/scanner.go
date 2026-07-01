// Package scanner periodically pulls the most active pools across chains,
// runs the liquidity-range-to-volatility analyzer over each one, and keeps the
// latest results in memory for the API to serve.
package scanner

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/onchain"
	"github.com/filipekdick/lp-tracker-go/internal/position"
)

// Config controls a scan cycle.
type Config struct {
	Chains           []datasource.Chain
	PerChain         int           // pools to keep per chain
	Interval         time.Duration // informational cadence for manual pool scans
	PositionInterval time.Duration // how often to refresh the LP position + hedge

	// Junk-pool filter (applied to the already-fetched batch, no extra API cost).
	// MinTVLUSD drops pools with negligible reserves; MaxTurnover drops pools
	// whose 24h volume/TVL is implausibly high (the signature of wash trading,
	// which otherwise produces impossible fee APRs). Zero disables a filter.
	MinTVLUSD   float64
	MaxTurnover float64

	// MaxProbePools caps how many pools per scan are probed on chain for their
	// real active liquidity (the concentrated-APR projection). The highest-ranked
	// pools are probed first; the rest use the bounded pool-data estimate. Zero
	// disables on-chain probing.
	MaxProbePools int
}

// DefaultMinTVLUSD and DefaultMaxTurnover are the filter defaults when the env
// vars are unset. The TVL floor removes dust pools; the turnover cap removes
// wash-trade pools (legitimate large pools clear both comfortably).
const (
	DefaultMinTVLUSD   = 50_000.0
	DefaultMaxTurnover = 20.0
)

// filterPools drops pools below the TVL floor or above the turnover cap. A
// non-positive threshold disables that check. Turnover is 24h volume / TVL.
func filterPools(pools []datasource.RawPool, minTVL, maxTurnover float64) []datasource.RawPool {
	out := pools[:0:0]
	for _, p := range pools {
		if minTVL > 0 && p.TVLUSD < minTVL {
			continue
		}
		if maxTurnover > 0 && p.TVLUSD > 0 && p.Volume24hUSD/p.TVLUSD > maxTurnover {
			continue
		}
		out = append(out, p)
	}
	return out
}

// AnalyzedPool is a raw pool plus its analyzer result, as served to clients.
type AnalyzedPool struct {
	datasource.RawPool
	Analysis analyzer.Result `json:"analysis"`
}

// Snapshot is an immutable view of the most recent scan.
type Snapshot struct {
	Pools      []AnalyzedPool            `json:"pools"`
	Position   *position.TrackedPosition `json:"position,omitempty"`
	Source     string                    `json:"source"`
	LastScan   time.Time                 `json:"lastScan"`
	NextScan   time.Time                 `json:"nextScan"`
	Scanning   bool                      `json:"scanning"`
	DurationMS int64                     `json:"durationMs"`
	Error      string                    `json:"error,omitempty"`
}

// Scanner owns the scan loop and the latest snapshot.
type Scanner struct {
	cfg     Config
	source  datasource.Source
	implied datasource.ImpliedVolSource
	tracker position.Tracker
	prober  onchain.Prober

	mu   sync.RWMutex
	snap Snapshot

	trigger      chan struct{}
	failedChains []datasource.Chain
}

// New builds a Scanner. implied may be nil to skip options-implied volatility,
// tracker may be nil to disable single-position tracking, and prober may be nil
// (or a onchain.NoopProber) to disable on-chain liquidity probing — pools then
// use the bounded pool-data estimate for their concentrated-APR projection.
func New(cfg Config, source datasource.Source, implied datasource.ImpliedVolSource, tracker position.Tracker, prober onchain.Prober) *Scanner {
	if prober == nil {
		prober = onchain.NoopProber{}
	}
	if cfg.PerChain <= 0 {
		cfg.PerChain = 10
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Minute
	}
	if cfg.PositionInterval <= 0 {
		cfg.PositionInterval = 3 * time.Minute
	}
	if len(cfg.Chains) == 0 {
		cfg.Chains = datasource.DefaultChains
	}
	return &Scanner{
		cfg:     cfg,
		source:  source,
		implied: implied,
		tracker: tracker,
		prober:  prober,
		snap:    Snapshot{Source: source.Name()},
		trigger: make(chan struct{}, 1),
	}
}

// Snapshot returns the latest results.
func (s *Scanner) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

// Config exposes the scanner configuration (chains, interval, per-chain count).
func (s *Scanner) Config() Config { return s.cfg }

// TriggerScan requests an immediate out-of-band scan. It is non-blocking; if a
// scan is already queued the request coalesces.
func (s *Scanner) TriggerScan() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Run starts two independent loops and blocks until ctx is cancelled: a fast
// loop that refreshes the LP position and Binance hedge, and a trigger-only
// pool scan that hits GeckoTerminal once on startup and thereafter only when
// the user clicks "Scan now" (TriggerScan). The two never share a timer, so the
// hedge stays tight without burning GeckoTerminal quota on every tick.
func (s *Scanner) Run(ctx context.Context) {
	go s.runPositionLoop(ctx)
	s.runPoolScanLoop(ctx)
}

// runPositionLoop refreshes the position + hedge immediately and then on every
// PositionInterval tick, independent of any pool scan.
func (s *Scanner) runPositionLoop(ctx context.Context) {
	s.refreshPosition(ctx)

	ticker := time.NewTicker(s.cfg.PositionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshPosition(ctx)
		}
	}
}

// runPoolScanLoop runs the GeckoTerminal pool scan once on startup (so the table
// is populated on first load) and thereafter only on manual triggers. Failed
// chains are retried on their own slow ticker. There is deliberately no
// automatic pool-scan ticker.
func (s *Scanner) runPoolScanLoop(ctx context.Context) {
	s.scanPools(ctx)

	retryTicker := time.NewTicker(30 * time.Second)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-retryTicker.C:
			s.retryFailed(ctx)
		case <-s.trigger:
			s.scanPools(ctx)
		}
	}
}

// refreshPosition reads the on-chain LP state and syncs the Binance hedge,
// updating only the tracked position. It never touches the pool-scan
// bookkeeping (Scanning / LastScan / failedChains).
func (s *Scanner) refreshPosition(ctx context.Context) {
	tracked := s.trackPosition(ctx)
	if tracked != nil {
		s.mu.Lock()
		s.snap.Position = tracked
		s.mu.Unlock()
	}
}

// scanPools runs the GeckoTerminal chain scan and is the only thing that drives
// the Scanning flag and the scan timestamps. It does not touch the position.
func (s *Scanner) scanPools(ctx context.Context) {
	s.setScanning(true)
	start := time.Now()

	s.mu.Lock()
	s.failedChains = nil
	s.mu.Unlock()

	// One on-chain probe budget for the whole scan; top-ranked pools per chain
	// consume it first.
	probeBudget := s.cfg.MaxProbePools

	for _, chain := range s.cfg.Chains {
		if ctx.Err() != nil {
			s.setScanning(false)
			return
		}
		raw, err := s.source.TopPools(ctx, []datasource.Chain{chain}, s.cfg.PerChain)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(strings.ToLower(errStr), "timeout") || strings.Contains(strings.ToLower(errStr), "deadline exceeded") {
				log.Printf("[scanner] timeout for chain %s: %v", chain.Slug, err)
			} else if strings.Contains(errStr, "429") || strings.Contains(strings.ToLower(errStr), "too many requests") {
				log.Printf("[scanner] rate limited for chain %s: %v", chain.Slug, err)
			} else {
				log.Printf("[scanner] scan failed for chain %s: %v", chain.Slug, err)
			}
			s.mu.Lock()
			s.failedChains = append(s.failedChains, chain)
			s.snap.Error = fmt.Sprintf("failed to scan chains: %s", formatChainSlugs(s.failedChains))
			s.mu.Unlock()
			continue
		}

		newAnalyzed := s.analyze(ctx, filterPools(raw, s.cfg.MinTVLUSD, s.cfg.MaxTurnover), &probeBudget)

		s.mu.Lock()
		// Merge results progressively
		var merged []AnalyzedPool
		for _, p := range s.snap.Pools {
			if p.ChainSlug != chain.Slug {
				merged = append(merged, p)
			}
		}
		merged = append(merged, newAnalyzed...)

		sort.SliceStable(merged, func(i, j int) bool {
			return merged[i].Analysis.Score > merged[j].Analysis.Score
		})

		s.snap.Pools = merged
		s.snap.Source = s.source.Name()
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.snap.Scanning = false
	s.snap.LastScan = time.Now()
	// Pool scanning is manual-only, so there is no scheduled next scan.
	s.snap.NextScan = time.Time{}
	s.snap.DurationMS = time.Since(start).Milliseconds()
	if len(s.failedChains) > 0 {
		if len(s.snap.Pools) == 0 {
			s.snap.Error = "all chains failed to scan"
		} else {
			s.snap.Error = fmt.Sprintf("failed to scan chains: %s", formatChainSlugs(s.failedChains))
		}
	} else {
		s.snap.Error = ""
	}
	s.mu.Unlock()
}

// CancelOrder delegates an order cancellation to the underlying position tracker.
func (s *Scanner) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	if s.tracker == nil {
		return fmt.Errorf("no position tracker configured")
	}
	return s.tracker.CancelOrder(ctx, symbol, orderID)
}

// TrackedTokenIDs returns the LP position token IDs currently tracked, or nil
// when no tracker is configured.
func (s *Scanner) TrackedTokenIDs() []int64 {
	if s.tracker == nil {
		return nil
	}
	return s.tracker.TokenIDs()
}

// SetTrackedTokenIDs replaces the tracked LP positions at runtime and refreshes
// the position immediately so the dashboard reflects the change on the next poll
// without waiting a full interval. It returns the resulting tracked set.
func (s *Scanner) SetTrackedTokenIDs(ctx context.Context, ids []int64) ([]int64, error) {
	if s.tracker == nil {
		return nil, fmt.Errorf("no position tracker configured")
	}
	s.tracker.SetTokenIDs(ids)
	s.refreshPosition(ctx)
	return s.tracker.TokenIDs(), nil
}

func (s *Scanner) retryFailed(ctx context.Context) {
	s.mu.Lock()
	failed := make([]datasource.Chain, len(s.failedChains))
	copy(failed, s.failedChains)
	s.mu.Unlock()

	// Position-tracking retries are handled by the fast position loop, so this
	// retry path only re-attempts failed GeckoTerminal chains.
	if len(failed) == 0 {
		return
	}

	s.setScanning(true)
	defer s.setScanning(false)

	log.Printf("[scanner] retrying failed chains: %s", formatChainSlugs(failed))

	var stillFailed []datasource.Chain
	probeBudget := s.cfg.MaxProbePools

	for _, chain := range failed {
		if ctx.Err() != nil {
			return
		}

		raw, err := s.source.TopPools(ctx, []datasource.Chain{chain}, s.cfg.PerChain)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(strings.ToLower(errStr), "timeout") || strings.Contains(strings.ToLower(errStr), "deadline exceeded") {
				log.Printf("[scanner] retry timeout for chain %s: %v", chain.Slug, err)
			} else if strings.Contains(errStr, "429") || strings.Contains(strings.ToLower(errStr), "too many requests") {
				log.Printf("[scanner] retry rate limited for chain %s: %v", chain.Slug, err)
			} else {
				log.Printf("[scanner] retry failed for chain %s: %v", chain.Slug, err)
			}
			stillFailed = append(stillFailed, chain)
			continue
		}

		newAnalyzed := s.analyze(ctx, filterPools(raw, s.cfg.MinTVLUSD, s.cfg.MaxTurnover), &probeBudget)

		s.mu.Lock()
		var merged []AnalyzedPool
		for _, p := range s.snap.Pools {
			if p.ChainSlug != chain.Slug {
				merged = append(merged, p)
			}
		}
		merged = append(merged, newAnalyzed...)

		sort.SliceStable(merged, func(i, j int) bool {
			return merged[i].Analysis.Score > merged[j].Analysis.Score
		})

		s.snap.Pools = merged
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.failedChains = stillFailed
	if len(s.failedChains) > 0 {
		if len(s.snap.Pools) == 0 {
			s.snap.Error = "all chains failed to scan"
		} else {
			s.snap.Error = fmt.Sprintf("failed to scan chains: %s", formatChainSlugs(s.failedChains))
		}
	} else {
		s.snap.Error = ""
	}
	s.mu.Unlock()
}

func formatChainSlugs(chains []datasource.Chain) string {
	var slugs []string
	for _, c := range chains {
		slugs = append(slugs, c.Slug)
	}
	return strings.Join(slugs, ", ")
}

// trackPosition refreshes the tracked LP/hedge, returning nil when no tracker
// is configured. A tracking failure is logged but does not abort the scan; the
// previous snapshot's position is retained.
func (s *Scanner) trackPosition(ctx context.Context) *position.TrackedPosition {
	if s.tracker == nil {
		return nil
	}
	tp, err := s.tracker.Track(ctx)
	if err != nil {
		log.Printf("[scanner] position tracking failed: %v", err)
		s.mu.RLock()
		prev := s.snap.Position
		s.mu.RUnlock()
		if prev != nil {
			return prev
		}
		return &position.TrackedPosition{Source: s.tracker.Name(), Error: err.Error()}
	}
	return &tp
}

// analyze runs the model over every raw pool, sorts the results by score (most
// attractive first), and attaches the concentrated-APR projection — probing the
// highest-ranked pools on chain for their real active liquidity until the
// per-scan budget is spent, with the rest using the bounded pool-data estimate.
func (s *Scanner) analyze(ctx context.Context, raw []datasource.RawPool, probeBudget *int) []AnalyzedPool {
	pools := make([]AnalyzedPool, 0, len(raw))
	for _, rp := range raw {
		in := analyzer.Input{
			FeeTier:        rp.FeeTier,
			TVLUSD:         rp.TVLUSD,
			Volume24hUSD:   rp.Volume24hUSD,
			Closes:         rp.Closes,
			PeriodsPerYear: rp.PeriodsPerYear,
			Bars:           rp.OHLC,
		}
		if s.implied != nil {
			if iv, ok := s.implied.ImpliedVol(ctx, rp.BaseSymbol); ok {
				in.ImpliedVol = iv
				in.HasImplied = true
			}
		}
		pools = append(pools, AnalyzedPool{RawPool: rp, Analysis: analyzer.Analyze(in)})
	}

	sort.SliceStable(pools, func(i, j int) bool {
		return pools[i].Analysis.Score > pools[j].Analysis.Score
	})

	// Attach the concentrated-APR projection, top-ranked first so the limited
	// on-chain probe budget goes to the most attractive pools.
	for i := range pools {
		rp := pools[i].RawPool
		allowProbe := probeBudget != nil && *probeBudget > 0
		sim := analyzer.ConcentratedProjection(ctx, s.prober, analyzer.ConcentratedInput{
			ChainSlug:    rp.ChainSlug,
			PoolAddress:  rp.Address,
			DEX:          rp.DEX,
			FeeTier:      rp.FeeTier,
			TVLUSD:       rp.TVLUSD,
			Volume24hUSD: rp.Volume24hUSD,
			RealizedVol:  pools[i].Analysis.RealizedVol,
			BaseUSD:      rp.PriceUSD,
			QuoteUSD:     rp.QuotePriceUSD,
		}, allowProbe)
		if sim != nil {
			pools[i].Analysis.ConcentratedSim = sim
			if sim.Source == "onchain" && probeBudget != nil {
				*probeBudget--
			}
		}
	}
	return pools
}

func (s *Scanner) setScanning(v bool) {
	s.mu.Lock()
	s.snap.Scanning = v
	s.mu.Unlock()
}
