// Package metrics counts actual outbound market-data requests and cache hits.
package metrics

import "sync"

var mu sync.Mutex
var counts = map[string]uint64{}

func Inc(name string) { mu.Lock(); counts[name]++; mu.Unlock() }
func Snapshot() map[string]uint64 {
	mu.Lock()
	defer mu.Unlock()
	out := map[string]uint64{}
	for k, v := range counts {
		out[k] = v
	}
	return out
}
