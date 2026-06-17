// Package api exposes the scanner's results over HTTP as JSON and serves the
// dashboard frontend.
package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/filipekdick/lp-tracker-go/internal/datasource"
	"github.com/filipekdick/lp-tracker-go/internal/scanner"
)

// Server wires the scanner and static assets into an http.Handler.
type Server struct {
	scanner *scanner.Scanner
	static  fs.FS
}

// New builds an API server. static is the filesystem holding the frontend
// assets (index.html, app.js, styles.css); it may be nil to disable the UI.
func New(sc *scanner.Scanner, static fs.FS) *Server {
	return &Server{scanner: sc, static: static}
}

// Handler returns the configured router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/meta", s.handleMeta)
	mux.HandleFunc("/api/pools", s.handlePools)
	mux.HandleFunc("/api/pools/", s.handlePoolDetail)
	mux.HandleFunc("/api/position", s.handlePosition)
	mux.HandleFunc("/api/scan", s.handleScan)

	if s.static != nil {
		mux.Handle("/", http.FileServer(http.FS(s.static)))
	}
	return logging(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

// metaResponse describes the scanner configuration for the UI.
type metaResponse struct {
	Source   string             `json:"source"`
	Interval string             `json:"interval"`
	PerChain int                `json:"perChain"`
	Chains   []datasource.Chain `json:"chains"`
	LastScan time.Time          `json:"lastScan"`
	NextScan time.Time          `json:"nextScan"`
	Scanning bool               `json:"scanning"`
	Counts   map[string]int     `json:"counts"`
}

func (s *Server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	cfg := s.scanner.Config()
	snap := s.scanner.Snapshot()

	counts := map[string]int{"attractive": 0, "fair": 0, "unattractive": 0, "unknown": 0}
	for _, p := range snap.Pools {
		counts[string(p.Analysis.Verdict)]++
	}

	writeJSON(w, http.StatusOK, metaResponse{
		Source:   snap.Source,
		Interval: cfg.Interval.String(),
		PerChain: cfg.PerChain,
		Chains:   cfg.Chains,
		LastScan: snap.LastScan,
		NextScan: snap.NextScan,
		Scanning: snap.Scanning,
		Counts:   counts,
	})
}

func (s *Server) handlePools(w http.ResponseWriter, r *http.Request) {
	snap := s.scanner.Snapshot()
	pools := snap.Pools

	// Optional filters: ?chainKind=L1|L2, ?chain=base, ?verdict=attractive
	q := r.URL.Query()
	if kind := q.Get("chainKind"); kind != "" {
		pools = filter(pools, func(p scanner.AnalyzedPool) bool {
			return string(p.ChainKind) == kind
		})
	}
	if chain := q.Get("chain"); chain != "" {
		pools = filter(pools, func(p scanner.AnalyzedPool) bool {
			return strings.EqualFold(p.ChainSlug, chain) || strings.EqualFold(p.Chain, chain)
		})
	}
	if verdict := q.Get("verdict"); verdict != "" {
		pools = filter(pools, func(p scanner.AnalyzedPool) bool {
			return string(p.Analysis.Verdict) == verdict
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":   snap.Source,
		"lastScan": snap.LastScan,
		"nextScan": snap.NextScan,
		"scanning": snap.Scanning,
		"error":    snap.Error,
		"count":    len(pools),
		"pools":    pools,
	})
}

// handlePoolDetail serves a single pool by /api/pools/{chainSlug}/{address}.
func (s *Server) handlePoolDetail(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/pools/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected /api/pools/{chain}/{address}"})
		return
	}
	chain, address := parts[0], parts[1]

	for _, p := range s.scanner.Snapshot().Pools {
		if strings.EqualFold(p.ChainSlug, chain) && strings.EqualFold(p.Address, address) {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "pool not found"})
}

// handlePosition serves the currently tracked LP position and its hedge.
func (s *Server) handlePosition(w http.ResponseWriter, _ *http.Request) {
	pos := s.scanner.Snapshot().Position
	if pos == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tracking": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracking": true, "position": pos})
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	s.scanner.TriggerScan()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan triggered"})
}

func filter(in []scanner.AnalyzedPool, keep func(scanner.AnalyzedPool) bool) []scanner.AnalyzedPool {
	out := make([]scanner.AnalyzedPool, 0, len(in))
	for _, p := range in {
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// logging records API request paths and latency; static asset hits are skipped
// to keep the log readable.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
		}
	})
}
