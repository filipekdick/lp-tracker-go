package marketdata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type brokenStore struct{}

func (brokenStore) LatestPrice(context.Context, string, time.Duration) (float64, time.Time, bool, error) {
	return 0, time.Time{}, false, errors.New("db offline")
}
func (brokenStore) SavePrice(context.Context, string, string, float64, time.Time) error {
	return errors.New("db offline")
}

func TestQuoteCacheExpirationAndDatabaseFailure(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			http.Error(w, "offline", 503)
			return
		}
		w.Write([]byte(`{"markPrice":"2000"}`))
	}))
	defer server.Close()
	b := NewBinanceMarkPrices(server.URL, brokenStore{}, time.Minute)
	first, err := b.Quote(context.Background(), "ETHUSDT")
	if err != nil || first.Price != 2000 {
		t.Fatal(first, err)
	}
	second, err := b.Quote(context.Background(), "ETHUSDT")
	if err != nil || !second.Cached || second.ObservedAt != first.ObservedAt || calls.Load() != 1 {
		t.Fatal("cache lost timestamp or requested again")
	}
	b.mu.Lock()
	old := b.quotes["ETHUSDT"]
	old.ObservedAt = time.Now().Add(-time.Hour)
	b.quotes["ETHUSDT"] = old
	b.mu.Unlock()
	fail.Store(true)
	if _, err = b.Quote(context.Background(), "ETHUSDT"); err == nil {
		t.Fatal("expired quote silently used")
	}
	if calls.Load() != 2 {
		t.Fatal("expired quote did not refresh")
	}
}
