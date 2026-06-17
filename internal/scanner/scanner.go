// Package scanner periodically pulls the most active pools across chains,
// runs the liquidity-range-to-volatility analyzer over each one, and keeps the
// latest results in memory for the API to serve.
package scanner

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
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
	Pools      []AnalyzedPool `json:"pools"`
	Source     string         `json:"source"`
	LastScan   time.Time      `json:"lastScan"`
	NextScan   time.Time      `json:"nextScan"`
	Scanning   bool           `json:"scanning"`
	DurationMS int64          `json:"durationMs"`
	Error      string         `json:"error,omitempty"`
}

// Scanner owns the scan loop and the latest snapshot.
type Scanner struct {
	cfg     Config
	source  datasource.Source
	implied datasource.ImpliedVolSource

	mu   sync.RWMutex
	snap Snapshot

	trigger chan struct{}
}

// New builds a Scanner. implied may be nil to skip options-implied volatility.
func New(cfg Config, source datasource.Source, implied datasource.ImpliedVolSource) *Scanner {
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scanOnce(ctx)
		case <-s.trigger:
			s.scanOnce(ctx)
			ticker.Reset(s.cfg.Interval)
		}
	}
}

func (s *Scanner) scanOnce(ctx context.Context) {
	s.setScanning(true)
	start := time.Now()

	raw, err := s.source.TopPools(ctx, s.cfg.Chains, s.cfg.PerChain)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.snap.Scanning = false
	s.snap.LastScan = time.Now()
	s.snap.NextScan = s.snap.LastScan.Add(s.cfg.Interval)
	s.snap.DurationMS = time.Since(start).Milliseconds()
	s.snap.Source = s.source.Name()

	if err != nil {
		s.snap.Error = err.Error()
		return
	}
	s.snap.Error = ""
	s.snap.Pools = s.analyze(ctx, raw)
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
