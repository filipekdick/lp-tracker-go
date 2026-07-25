package position

import (
	"context"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/datasource"
)

// PoolSnapshotStore persists complete pool responses, including their existing
// OHLCV history, so normal position refreshes do not consume GeckoTerminal
// quota. It is intentionally an interface to keep the tracker testable.
type PoolSnapshotStore interface {
	LoadPool(ctx context.Context, chainSlug, address string, maxAge time.Duration) (datasource.RawPool, bool, error)
	SavePool(ctx context.Context, pool datasource.RawPool) error
}

type cachedPoolPricer struct {
	upstream poolPricer
	store    PoolSnapshotStore
	ttl      time.Duration
}

// NewCachedPoolPricer wraps an upstream pool provider with persisted snapshots.
func NewCachedPoolPricer(upstream poolPricer, store PoolSnapshotStore, ttl time.Duration) poolPricer {
	if store == nil {
		return upstream
	}
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	return &cachedPoolPricer{upstream: upstream, store: store, ttl: ttl}
}

func (c *cachedPoolPricer) PoolByAddress(ctx context.Context, chain datasource.Chain, address string) (datasource.RawPool, error) {
	if pool, ok, err := c.store.LoadPool(ctx, chain.Slug, address, c.ttl); err == nil && ok {
		return pool, nil
	}
	pool, err := c.upstream.PoolByAddress(ctx, chain, address)
	if err != nil {
		return datasource.RawPool{}, err
	}
	// Persistence failure must not prevent an otherwise valid valuation.
	_ = c.store.SavePool(ctx, pool)
	return pool, nil
}
