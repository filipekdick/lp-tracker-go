// Package store persists market data used by the tracker.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/analyzer"
	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type poolSnapshot struct {
	Pool datasource.RawPool `json:"pool"`
	// RawPool excludes OHLC from its normal JSON representation to keep API
	// responses small; retain it in PostgreSQL for cached volatility analysis.
	OHLC []analyzer.OHLCV `json:"ohlc"`
}

// Postgres stores append-only price observations and the latest complete pool
// snapshot. It is deliberately small: migrations are applied at startup.
type Postgres struct {
	db *sql.DB
}

// Open connects and creates the tables required by the tracker.
func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	p := &Postgres{db: db}
	if err := p.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return p, nil
}

func (p *Postgres) migrate(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS market_prices (
  symbol TEXT NOT NULL,
  source TEXT NOT NULL,
  price_usd DOUBLE PRECISION NOT NULL CHECK (price_usd > 0),
  observed_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (symbol, source, observed_at)
);
CREATE INDEX IF NOT EXISTS market_prices_latest_idx ON market_prices (symbol, observed_at DESC);
CREATE TABLE IF NOT EXISTS pool_snapshots (
  chain_slug TEXT NOT NULL,
  address TEXT NOT NULL,
  data JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (chain_slug, address)
);`)
	if err != nil {
		return fmt.Errorf("migrate postgres: %w", err)
	}
	return nil
}

func (p *Postgres) Close() error { return p.db.Close() }

// LatestPrice returns a price no older than maxAge.
func (p *Postgres) LatestPrice(ctx context.Context, symbol string, maxAge time.Duration) (float64, bool, error) {
	var price float64
	err := p.db.QueryRowContext(ctx, `
SELECT price_usd FROM market_prices
WHERE symbol = $1 AND observed_at >= $2
ORDER BY observed_at DESC LIMIT 1`, symbol, time.Now().Add(-maxAge)).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("load price %s: %w", symbol, err)
	}
	return price, true, nil
}

func (p *Postgres) SavePrice(ctx context.Context, symbol, source string, price float64, observedAt time.Time) error {
	if price <= 0 {
		return nil
	}
	_, err := p.db.ExecContext(ctx, `INSERT INTO market_prices (symbol, source, price_usd, observed_at) VALUES ($1, $2, $3, $4)`, symbol, source, price, observedAt)
	if err != nil {
		return fmt.Errorf("save price %s: %w", symbol, err)
	}
	return nil
}

func (p *Postgres) LoadPool(ctx context.Context, chainSlug, address string, maxAge time.Duration) (datasource.RawPool, bool, error) {
	var data []byte
	err := p.db.QueryRowContext(ctx, `SELECT data FROM pool_snapshots WHERE chain_slug = $1 AND address = $2 AND updated_at >= $3`, chainSlug, address, time.Now().Add(-maxAge)).Scan(&data)
	if err == sql.ErrNoRows {
		return datasource.RawPool{}, false, nil
	}
	if err != nil {
		return datasource.RawPool{}, false, fmt.Errorf("load pool %s: %w", address, err)
	}
	var snapshot poolSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return datasource.RawPool{}, false, fmt.Errorf("decode pool %s: %w", address, err)
	}
	snapshot.Pool.OHLC = snapshot.OHLC
	return snapshot.Pool, true, nil
}

func (p *Postgres) SavePool(ctx context.Context, pool datasource.RawPool) error {
	data, err := json.Marshal(poolSnapshot{Pool: pool, OHLC: pool.OHLC})
	if err != nil {
		return fmt.Errorf("encode pool %s: %w", pool.Address, err)
	}
	_, err = p.db.ExecContext(ctx, `
INSERT INTO pool_snapshots (chain_slug, address, data, updated_at) VALUES ($1, $2, $3, NOW())
ON CONFLICT (chain_slug, address) DO UPDATE SET data = EXCLUDED.data, updated_at = EXCLUDED.updated_at`, pool.ChainSlug, pool.Address, data)
	if err != nil {
		return fmt.Errorf("save pool %s: %w", pool.Address, err)
	}
	return nil
}
