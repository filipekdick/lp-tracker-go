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
	"github.com/filipekdick/lp-tracker-go/internal/position"
)

// Config controls a scan cycle.
type Config struct {
	Chains   []datasource.Chain
	PerChain int           // pools to keep per chain
	Interval time.Duration // wait between automatic scans
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

	mu   sync.RWMutex
	snap Snapshot

	trigger      chan struct{}
	failedChains []datasource.Chain
}

// New builds a Scanner. implied may be nil to skip options-implied volatility,
// and tracker may be nil to disable single-position tracking.
func New(cfg Config, source datasource.Source, implied datasource.ImpliedVolSource, tracker position.Tracker) *Scanner {
	if cfg.PerChain <= 0 {
		cfg.PerChain = 10
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Minute
	}
	if len(cfg.Chains) == 0 {
		cfg.Chains = datasource.DefaultChains
	}
	return &Scanner{
		cfg:     cfg,
		source:  source,
		implied: implied,
		tracker: tracker,
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

// Run executes one scan immediately, then loops on the configured interval
// until ctx is cancelled. Manual triggers run between ticks.
func (s *Scanner) Run(ctx context.Context) {
	s.scanOnce(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	retryTicker := time.NewTicker(30 * time.Second)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scanOnce(ctx)
		case <-retryTicker.C:
			s.retryFailed(ctx)
		case <-s.trigger:
			s.scanOnce(ctx)
			ticker.Reset(s.cfg.Interval)
		}
	}
}

func (s *Scanner) scanOnce(ctx context.Context) {
	s.setScanning(true)
	start := time.Now()

	s.mu.Lock()
	s.failedChains = nil
	s.mu.Unlock()

	// Run position tracking concurrently in its own goroutine
	go func() {
		tracked := s.trackPosition(ctx)
		if tracked != nil {
			s.mu.Lock()
			s.snap.Position = tracked
			s.mu.Unlock()
		}
	}()

	for _, chain := range s.cfg.Chains {
		if ctx.Err() != nil {
			s.setScanning(false)
			return
		}
		raw, err := s.source.TopPools(ctx, []datasource.Chain{chain}, s.cfg.PerChain)
		if err != nil {
			log.Printf("[scanner] scan failed for chain %s: %v", chain.Slug, err)
			s.mu.Lock()
			s.failedChains = append(s.failedChains, chain)
			s.snap.Error = fmt.Sprintf("failed to scan chains: %s", formatChainSlugs(s.failedChains))
			s.mu.Unlock()
			continue
		}

		newAnalyzed := s.analyze(ctx, raw)

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
	s.snap.NextScan = s.snap.LastScan.Add(s.cfg.Interval)
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

func (s *Scanner) retryFailed(ctx context.Context) {
	s.mu.Lock()
	failed := make([]datasource.Chain, len(s.failedChains))
	copy(failed, s.failedChains)
	posErr := s.tracker != nil && (s.snap.Position == nil || s.snap.Position.Error != "")
	s.mu.Unlock()

	if posErr {
		log.Println("[scanner] retrying failed position tracking...")
		go func() {
			tracked := s.trackPosition(ctx)
			if tracked != nil {
				s.mu.Lock()
				s.snap.Position = tracked
				s.mu.Unlock()
			}
		}()
	}

	if len(failed) == 0 {
		return
	}

	s.setScanning(true)
	defer s.setScanning(false)

	log.Printf("[scanner] retrying failed chains: %s", formatChainSlugs(failed))

	var stillFailed []datasource.Chain

	for _, chain := range failed {
		if ctx.Err() != nil {
			return
		}

		raw, err := s.source.TopPools(ctx, []datasource.Chain{chain}, s.cfg.PerChain)
		if err != nil {
			log.Printf("[scanner] retry failed for chain %s: %v", chain.Slug, err)
			stillFailed = append(stillFailed, chain)
			continue
		}

		newAnalyzed := s.analyze(ctx, raw)

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

// analyze runs the model over every raw pool and returns the results sorted by
// score (most attractive first).
func (s *Scanner) analyze(ctx context.Context, raw []datasource.RawPool) []AnalyzedPool {
	pools := make([]AnalyzedPool, 0, len(raw))
	for _, rp := range raw {
		in := analyzer.Input{
			FeeTier:        rp.FeeTier,
			TVLUSD:         rp.TVLUSD,
			Volume24hUSD:   rp.Volume24hUSD,
			Closes:         rp.Closes,
			PeriodsPerYear: rp.PeriodsPerYear,
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
	return pools
}

func (s *Scanner) setScanning(v bool) {
	s.mu.Lock()
	s.snap.Scanning = v
	s.mu.Unlock()
}
