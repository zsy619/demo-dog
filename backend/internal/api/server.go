package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/ingest"
	"github.com/zsy619/demo-dog/backend/internal/store"
	"github.com/zsy619/demo-dog/backend/internal/stream"
	"github.com/zsy619/demo-dog/backend/internal/tenants"
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

	// alertsEngine evaluates SLO burn-rate rules and fires webhooks.
	alerts *alertsEngine

	// tenants is the optional in-memory tenant registry. nil when
	// the server runs in single-tenant mode.
	tenants *tenants.Registry

	// mux is the top-level http.ServeMux. Exposed so add-on endpoints
	// (pprof, probes) can be mounted after construction.
	mux *http.ServeMux

	// pprofPrefix / pprofToken are set by MountPProf and consulted by
	// the chain constructed in Handler() so pprof lives OUTSIDE the
	// auth + audit middleware (no token = no metrics, no audit spam).
	pprofPrefix string
	pprofToken  string

	// pprofHandler is the assembled sub-mux exposed via the auth-
	// bypass layer. Lazily constructed in Handler().
	pprofHandler http.Handler
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
		alerts:      newAlertsEngine(s),
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
	s.mux = http.NewServeMux()
	mux := s.mux

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
	// OTLP/HTTP standard transport (https://opentelemetry.io/docs/specs/otlp/#otlphttp).
	// Each signal has its own endpoint so collectors / agents that
	// fan out by type find the path they expect.
	mux.HandleFunc("/v1/logs", s.handleOTLPHTTPLogs)
	mux.HandleFunc("/v1/metrics", s.handleOTLPHTTPMetrics)
	mux.HandleFunc("/v1/traces", s.handleOTLPHTTPTraces)
	// PromQL endpoint for Grafana / Alertmanager. Subset of
	// PromQL: selectors with label filters, sum/avg/count by (dim),
	// rate(metric[1m]), histogram_quantile(q, metric).
	mux.HandleFunc("/api/v1/query", s.handlePromQL)
	// Prometheus Remote Write 1.0 — accepts both /api/v1/write
	// (the canonical path) and /api/prom/write (the aliased one).
	// The protocol is documented at:
	// https://prometheus.io/docs/concepts/remote_write_spec/
	mux.HandleFunc("/api/v1/write", s.handlePromRemoteWrite)
	mux.HandleFunc("/api/prom/write", s.handlePromRemoteWrite)
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
	mux.HandleFunc("/api/probe", s.handleProbe)
	mux.HandleFunc("/api/alerts/rules", s.handleAlertsRules)
	mux.HandleFunc("/api/alerts/fires", s.handleAlertsFires)
	mux.HandleFunc("/api/tenants", s.handleTenantsDispatch)
	mux.HandleFunc("/api/tenants/", s.handleTenantsDispatch)
	mux.HandleFunc("/metrics", s.handlePromMetrics)

	// Layering (outer -> inner):
	//   withCORS -> audit -> rateLimit -> selfTrace -> latency ->
	//   (pprof + auth.Middleware) -> applyRoleGates -> mux
	//
	// auth.Middleware runs BEFORE the role gate so it has a chance
	// to stamp X-Dog-Role on the request header. pprof routes are
	// mounted in their own layer so a /debug/pprof/* request never
	// reaches the auth gate.
	gated := s.applyRoleGates(mux)
	h := s.auth.Middleware(s.authM,
		"/api/health", "/metrics", "/api/probe",
	)(gated)
	if s.pprofToken != "" {
		h = s.buildPProfMux(h)
	}
	if s.rateLimiter != nil {
		h = s.rateLimiter.Middleware()(h)
	}
	if s.auditLog != nil {
		h = AuditMiddleware(s.auditLog, false)(h)
	}
	h = s.selfTraceMiddleware(h)
	h = perHandlerLatency(h)
	return s.withCORS(withLogging(h))
}

// perHandlerLatency wraps an http.Handler in a histogram that records
// the wall-clock duration of every request, labelled by HTTP method
// and (rough) route. The "route" is the URL path stripped of any
// query string and trailing service identifier, so the metric stays
// low-cardinality even with many distinct services.
//
// Exposed via /metrics under the name `dog_request_duration_seconds`.
func perHandlerLatency(next http.Handler) http.Handler {
	// Use a fixed bucket boundary set tuned for an observability
	// backend: 1 ms ... 30 s. Buckets are global (not per-route) to
	// keep the metric cardinality bounded.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start).Seconds()
		route := trimRoute(r.URL.Path)
		requestDuration.WithLabelValues(r.Method, route).Observe(dur)
	})
}

// trimRoute collapses noisy path segments to keep metric cardinality
// predictable: service-id-like segments are replaced with `{name}`
// and span-id-like hex strings with `{id}`. Anything else is left as
// is.
func trimRoute(p string) string {
	out := make([]byte, 0, len(p))
	inName := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '/' {
			out = append(out, c)
			inName = true
			continue
		}
		if inName {
			// Detect hex-only segment >= 16 chars (span / trace IDs).
			j := i
			for j < len(p) && p[j] != '/' {
				j++
			}
			seg := p[i:j]
			switch {
			case isHex(seg) && len(seg) >= 16:
				out = append(out, []byte("{id}")...)
			case len(seg) > 0 && seg != "api":
				out = append(out, []byte("{name}")...)
			default:
				out = append(out, []byte(seg)...)
			}
			i = j - 1
			inName = false
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') && !(c >= 'A' && c <= 'F') {
			return false
		}
	}
	return len(s) > 0
}

// MountPProf registers the net/http/pprof handlers at the given
// prefix, gated by a token query parameter. The token check is run
// before any pprof handler so a leaked URL alone is insufficient.
func (s *Server) MountPProf(prefix, token string) {
	s.pprofPrefix = prefix
	s.pprofToken = token
}

// SetTenants wires the tenant registry to the server. Once a registry
// is attached, /api/tenants endpoints become live.
func (s *Server) SetTenants(reg *tenants.Registry) {
	s.tenants = reg
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
		"/api/tenants":      true,
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
	// Tenant filter: prefer the auth-bound tenant (X-Dog-Tenant), fall
	// back to ?tenant=... (used by platform admins to impersonate).
	tenant := resolveTenant(r)
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
	sum, ok := s.store.GetService(resolveTenant(r), name)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("service not found"))
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// WrapWithSelfTrace enables self-tracing. When enabled, every
// request through the server produces an OTLP span (POSTed to
// /api/ingest/otlp on loopback) so the collector can graph its own
// latency. The trace IDs are minted here; downstream SDKs that
// honour W3C tracecontext will stitch into the same tree.
func (s *Server) WrapWithSelfTrace(loopback string) {
	selfTraceMu.Lock()
	selfTraceEnabled = true
	selfTraceLoopback = loopback
	selfTraceMu.Unlock()
}

// handleProbe is a synthetic blackbox probe endpoint. It always
// returns 200 OK with a small JSON body that lists the engine stats.
// K8s readinessProbe and external uptime monitors hit this endpoint.
// No authentication is required so a misconfigured auth layer cannot
// take the collector offline.
func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	stats := s.store.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"uptime_seconds":    int(time.Since(s.started).Seconds()),
		"logs_accepted":     stats.LogsAccepted,
		"metrics_accepted":  stats.MetricsAccepted,
		"spans_accepted":    stats.SpansAccepted,
		"queries_served":    stats.QueriesServed,
	})
}

// buildPProfMux wraps `next` in a small mux that handles the
// configured /debug/pprof/* paths (each gated by the configured
// token) and falls through to next for everything else. Called from
// Handler() when MountPProf was invoked.
func (s *Server) buildPProfMux(next http.Handler) http.Handler {
	token := s.pprofToken
	prefix := s.pprofPrefix
	gate := func(real http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("token") != token {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			real(w, r)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc(prefix+"/", gate(pprof.Index))
	mux.HandleFunc(prefix+"/cmdline", gate(pprof.Cmdline))
	mux.HandleFunc(prefix+"/profile", gate(pprof.Profile))
	mux.HandleFunc(prefix+"/symbol", gate(pprof.Symbol))
	mux.HandleFunc(prefix+"/trace", gate(pprof.Trace))
	mux.HandleFunc(prefix+"/goroutine", gate(pprof.Handler("goroutine").ServeHTTP))
	mux.HandleFunc(prefix+"/heap", gate(pprof.Handler("heap").ServeHTTP))
	mux.HandleFunc(prefix+"/allocs", gate(pprof.Handler("allocs").ServeHTTP))
	mux.HandleFunc(prefix+"/block", gate(pprof.Handler("block").ServeHTTP))
	mux.HandleFunc(prefix+"/mutex", gate(pprof.Handler("mutex").ServeHTTP))
	mux.HandleFunc(prefix+"/threadcreate", gate(pprof.Handler("threadcreate").ServeHTTP))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, p := mux.Handler(r)
		if p == "" {
			next.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
