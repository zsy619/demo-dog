package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/ingest"
	"github.com/zsy619/demo-dog/backend/internal/store"
	"github.com/zsy619/demo-dog/backend/internal/stream"
)

// Server wires routes to the underlying services.
type Server struct {
	store   *store.Doris
	ingest  *ingest.Ingestor
	hub     *stream.Hub
	started time.Time

	rng   *rand.Rand
	rngMu sync.Mutex

	// datasources is a thread-safe registry of logical backends the
	// collector can route queries to. Plug a real Doris / ClickHouse
	// driver at startup via Server.Datasources().Add(...).
	datasources *datasourceRegistry
}

// New returns a new Server.
func New(s *store.Doris, in *ingest.Ingestor, hub *stream.Hub) *Server {
	return &Server{
		store:       s,
		ingest:      in,
		hub:         hub,
		datasources: newDatasourceRegistry(),
		started:     time.Now(),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Datasources exposes the datasource registry so callers (e.g. a
// driver plugin at startup) can register additional backends.
func (s *Server) Datasources() *datasourceRegistry {
	return s.datasources
}

// Handler returns the root http.Handler with all routes mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/services", s.handleServices)
	mux.HandleFunc("/api/services/", s.handleServiceDetail)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/datasources", s.handleDataSources)
	mux.HandleFunc("/api/dashboards", s.handleDashboards)
	mux.HandleFunc("/api/dashboards/", s.handleDashboardsPanels)
	mux.HandleFunc("/api/ingest/otlp", s.handleIngest)
	mux.HandleFunc("/api/ingest/otlp-json", s.handleIngest)
	mux.HandleFunc("/api/stream", s.handleStream)
	mux.HandleFunc("/api/seed", s.handleSeed)
	mux.HandleFunc("/api/seed/stream", s.handleSeedStream)
	mux.HandleFunc("/api/ingest/recent", s.handleRecentPayloads)
	mux.HandleFunc("/api/labels", s.handleLabelKeys)
	mux.HandleFunc("/api/service-map", s.handleServiceMap)
	mux.HandleFunc("/api/traces/", s.handleTrace)
	mux.HandleFunc("/api/qps", s.handleQPS)
	mux.HandleFunc("/api/histogram", s.handleHistogram)
	mux.HandleFunc("/api/histogram/otel", s.handleHistogramOTel)
	mux.HandleFunc("/api/severity", s.handleSeverity)
	mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/metric-names", s.handleMetricNames)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/metrics", s.handlePromMetrics)

	return withCORS(withLogging(mux))
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		fmt.Printf("[DOG] %-4s %-30s %dms\n", r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "[DOG] json encode:", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	st := s.store.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"uptime":  time.Since(s.started).String(),
		"engine":  st,
		"version": "demo-dog-0.1.0",
		"now":     time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	out := s.store.ListServices()
	writeJSON(w, http.StatusOK, map[string]any{
		"services": out,
		"count":    len(out),
	})
}

func (s *Server) handleServiceDetail(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/services/")
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New("missing service name"))
		return
	}
	// /api/services/{name}/detail -> drill-down payload (endpoints, errors, traces).
	if strings.HasSuffix(name, "/detail") {
		svc := strings.TrimSuffix(name, "/detail")
		det, ok := s.store.ServiceDetail(svc)
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("service not found"))
			return
		}
		writeJSON(w, http.StatusOK, det)
		return
	}
	sum, ok := s.store.GetService(name)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("service not found"))
		return
	}
	writeJSON(w, http.StatusOK, sum)
}
