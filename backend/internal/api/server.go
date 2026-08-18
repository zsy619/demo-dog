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

	// auth is the API-key registry. Empty by default (dev mode);
	// populated via the -api-keys flag or DOG_API_KEYS env var.
	auth  *APIKeyAuth
	authM AuthMode

	// allowedOrigins controls CORS. Empty slice = wildcard "*".
	// Populate via SetAllowedOrigins from main.
	allowedOrigins []string

	// rateLimiter is nil unless enabled via SetRateLimit. It uses a
	// per-IP token bucket and returns 429 with Retry-After when a
	// single client floods the server.
	rateLimiter *RateLimiter

	// auditLog records every write operation (and optionally reads)
	// for compliance + post-incident forensics. Created lazily on
	// first access; tests can swap it via SetAuditLog.
	auditLog *AuditLog
}

// New returns a new Server.
func New(s *store.Doris, in *ingest.Ingestor, hub *stream.Hub) *Server {
	return &Server{
		store:       s,
		ingest:      in,
		hub:         hub,
		datasources: newDatasourceRegistry(),
		auth:        NewAPIKeyAuth(),
		authM:       AuthModeOff,
		auditLog:    NewAuditLog(10_000),
		started:     time.Now(),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Audit returns the audit log so callers can configure capacity at
// startup or swap the implementation for tests.
func (s *Server) Audit() *AuditLog { return s.auditLog }

// SetAuditLog replaces the default audit log. Useful in tests or
// when wiring a remote sink.
func (s *Server) SetAuditLog(l *AuditLog) { s.auditLog = l }

// Datasources exposes the datasource registry so callers (e.g. a
// driver plugin at startup) can register additional backends.
func (s *Server) Datasources() *datasourceRegistry {
	return s.datasources
}

// Auth exposes the API key registry so callers can register keys at
// startup (or for tests). AuthMode() tells the middleware which mode
// to enforce.
func (s *Server) Auth() *APIKeyAuth    { return s.auth }
func (s *Server) AuthMode() AuthMode    { return s.authM }
func (s *Server) SetAuthMode(m AuthMode) { s.authM = m }

// SetAllowedOrigins restricts CORS Access-Control-Allow-Origin to the
// given host list. Empty list keeps the wildcard default. Origins are
// matched exactly (no scheme-relative quirks); set http://localhost:3000
// if you only want to allow the dev frontend.
func (s *Server) SetAllowedOrigins(origins []string) {
	s.allowedOrigins = origins
}

// SetRateLimit installs a per-IP token-bucket rate limiter. Pass
// rate=0 to disable.
func (s *Server) SetRateLimit(rate, burst float64) {
	if rate <= 0 {
		s.rateLimiter = nil
		return
	}
	s.rateLimiter = NewRateLimiter(rate, burst)
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
	mux.HandleFunc("/api/audit", s.handleAudit)
	mux.HandleFunc("/api/audit/stats", s.handleAuditStats)
	mux.HandleFunc("/api/keys", s.handleListKeys)
	mux.HandleFunc("/metrics", s.handlePromMetrics)

	// Layering (outer -> inner):
	//   withCORS -> audit -> rateLimit -> auth.Middleware ->
	//   applyRoleGates -> mux
	//
	// auth.Middleware runs BEFORE the role gate so it has a chance
	// to stamp X-Dog-Role on the request header. Anything outside
	// auth sees the headers intact.
	gated := s.applyRoleGates(mux)
	h := s.auth.Middleware(s.authM,
		"/api/health", "/metrics",
	)(gated)
	if s.rateLimiter != nil {
		h = s.rateLimiter.Middleware()(h)
	}
	if s.auditLog != nil {
		h = AuditMiddleware(s.auditLog, false)(h)
	}
	return s.withCORS(withLogging(h))
}

// applyRoleGates returns a handler that gates specific routes on role.
// Anything not in the gate list passes through unchanged.
func (s *Server) applyRoleGates(next http.Handler) http.Handler {
	adminOnly := map[string]bool{
		"/api/audit":       true,
		"/api/audit/stats": true,
		"/api/keys":         true,
		"/api/seed":         true,
		"/api/seed/stream":  true,
	}
	// writer+ so ingest is open to writers (default) and admin.
	writerOrUp := map[string]bool{
		"/api/ingest/otlp":      true,
		"/api/ingest/otlp-json": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if adminOnly[r.URL.Path] {
			RequireRole(RoleAdmin, next).ServeHTTP(w, r)
			return
		}
		if writerOrUp[r.URL.Path] && r.Method != http.MethodGet {
			RequireRole(RoleWriter, next).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withCORS(h http.Handler) http.Handler {
	allowed := s.allowedOrigins
	if len(allowed) == 0 {
		allowed = []string{"*"}
	}
	wildcard := len(allowed) == 1 && allowed[0] == "*"
	isAllowed := func(origin string) bool {
		if wildcard {
			return true
		}
		for _, a := range allowed {
			if a == origin {
				return true
			}
		}
		return false
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if isAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			} else if !wildcard {
				// Unknown origin — return no ACAO header so the
				// browser rejects the response.
				w.WriteHeader(http.StatusForbidden)
				return
			}
		} else if wildcard {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
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
	// Tenant filter via ?tenant=... query param. Empty string means
	// no filter (back-compat with single-tenant callers).
	tenant := r.URL.Query().Get("tenant")
	out := s.store.ListServices(tenant)
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
	sum, ok := s.store.GetService(r.URL.Query().Get("tenant"), name)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("service not found"))
		return
	}
	writeJSON(w, http.StatusOK, sum)
}
