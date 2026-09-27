package position

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/metrics"
)

// PoolSnapshotStore persists analysis inputs independently of valuation.
type PoolSnapshotStore interface {
	LoadPool(context.Context, string, string, time.Duration) (datasource.RawPool, bool, error)
	SavePool(context.Context, datasource.RawPool) error
}

type analysisEntry struct {
	pool    datasource.RawPool
	loading bool
	retryAt time.Time
}

type cachedPoolPricer struct {
	upstream poolPricer
	store    PoolSnapshotStore
	ttl      time.Duration
	mu       sync.Mutex
	entries  map[string]analysisEntry
}

// NewCachedPoolPricer returns a nonblocking analysis cache. Cold/expired entries
// refresh in the background, at most once per pool, with a bounded timeout and
// retry backoff. It works in memory even when PostgreSQL is disabled.
func NewCachedPoolPricer(upstream poolPricer, store PoolSnapshotStore, ttl time.Duration) poolPricer {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	return &cachedPoolPricer{upstream: upstream, store: store, ttl: ttl, entries: map[string]analysisEntry{}}
}

func (c *cachedPoolPricer) PoolByAddress(ctx context.Context, chain datasource.Chain, address string) (datasource.RawPool, error) {
	key := chain.Slug + ":" + address
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key]
	if !e.loading && time.Now().After(e.retryAt) && (e.pool.ObservedAt.IsZero() || time.Since(e.pool.ObservedAt) >= c.ttl) && ctx.Err() == nil {
		metrics.Inc("pool_analysis_refreshes")
		e.loading = true
		c.entries[key] = e
		go c.refresh(ctx, key, chain, address)
	}
	if e.pool.ObservedAt.IsZero() {
		return datasource.RawPool{}, fmt.Errorf("pool analysis pending")
	}
	if time.Since(e.pool.ObservedAt) >= c.ttl {
		return e.pool, fmt.Errorf("pool analysis stale; background refresh pending")
	}
	metrics.Inc("pool_analysis_cache_hits")
	return e.pool, nil
}

func (c *cachedPoolPricer) refresh(parent context.Context, key string, chain datasource.Chain, address string) {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	var pool datasource.RawPool
	var err error
	found := false
	if c.store != nil {
		pool, found, _ = c.store.LoadPool(ctx, chain.Slug, address, c.ttl)
	}
	// Older persisted records without observation time cannot claim freshness.
	if !found || pool.ObservedAt.IsZero() {
		pool, err = c.upstream.PoolByAddress(ctx, chain, address)
		if err == nil {
			pool.ObservedAt = time.Now()
			if c.store != nil {
				_ = c.store.SavePool(ctx, pool)
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key]
	e.loading = false
	if err == nil {
		e.pool = pool
	} else {
		metrics.Inc("pool_analysis_errors")
		e.retryAt = time.Now().Add(time.Minute)
	}
	c.entries[key] = e
}
