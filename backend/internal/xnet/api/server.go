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

	"github.com/zsy619/demo-dog/backend/internal/xsecure/auth"
	"github.com/zsy619/demo-dog/backend/internal/xdata/ingest"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
	"github.com/zsy619/demo-dog/backend/internal/xflow/stream"
	"github.com/zsy619/demo-dog/backend/internal/xdata/tenants"
)

// Server 将路由连接到各底层服务。
type Server struct {
	store   *store.Doris
	ingest  *ingest.Ingestor
	hub     *stream.Hub
	started time.Time

	rng   *rand.Rand
	rngMu sync.Mutex

	// datasources 是一个线程安全的逻辑后端注册表，
	// 采集器可向其路由查询。启动时通过 Server.Datasources().Add(...) 接入真实的 Doris / ClickHouse
	// driver。
	datasources *datasourceRegistry

	// auth 是 API-key 注册表。默认为空（开发模式）；
	// 通过 -api-keys 参数或 DOG_API_KEYS 环境变量填充。
	auth  *APIKeyAuth
	authM AuthMode

	// allowedOrigins 控制 CORS。空切片表示通配符 "*"。
	// 通过 main 中的 SetAllowedOrigins 进行填充。
	allowedOrigins []string

	// rateLimiter 在通过 SetRateLimit 启用之前为 nil。它使用
	// per-IP token bucket，当单个客户端
	// 大量请求打爆服务器时返回 429 并附 Retry-After。
	rateLimiter *RateLimiter

	// auditLog 记录每一次写入操作（可选地也记录读取），
	// 用于合规与事后取证。在
	// 首次访问时按需创建；测试可通过 SetAuditLog 替换它。
	auditLog *AuditLog

	// alertsEngine 评估 SLO 烧速率规则并触发 webhook。
	alerts *alertsEngine

	// tenants 是可选的内存租户注册表。当
	// 服务以单租户模式运行时为 nil。
	tenants *tenants.Registry

	// quota 是按租户的配额跟踪器（第 42 轮）。
	quota *QuotaTracker

	// limiter 是按 IP 的限流器（第 42 / 48 轮）。
	limiter *RateLimiter

	// breaker 注册表持有熔断器（第 47 轮）。
	breaker *BreakerRegistry

	// webhooks 是出站分发器（第 49 轮）。
	webhooks *WebhookDispatcher

	// retention 是按租户的留存管理器（第 50 轮）。
	retention *RetentionManager

	// adminKeys 拥有全局 API 密钥表（第 46 轮）。
	adminKeys *auth.AdminStore

	// replica 是至少一次复制的状态（第 38 轮）。
	replica *ReplicaStatus

	// oidc 是 OIDC 联邦提供方列表（第 41 轮）。
	oidc *OIDCRegistry

	// cfg 保存数据目录与管理端点所需的配置。
	cfg ServerConfig

	// mux 是顶层 http.ServeMux。暴露出来以便附加端点
	//（pprof、probes）可以在构造之后挂载。
	mux *http.ServeMux

	// pprofPrefix / pprofToken 由 MountPProf 设置，并在 Handler() 构造的
	// 链路中被引用，这样 pprof 就位于 auth + audit 中间件之外
	// (没有 token 就拿不到指标，也不会污染审计日志)。
	pprofPrefix string
	pprofToken  string

	// pprofHandler 是通过 auth-旁路层暴露的已组装子 mux。
	// 在 Handler() 中惰性构造。
	pprofHandler http.Handler

	// seriesCatalog 遍历内存中的指标缓冲，为 /api/v1/series 端点
	// 生成每个指标、每个 label 集合的基数信息。
	// 首次使用时惰性构造。
	seriesCatalog *store.SeriesCatalog
	seriesCatalogOnce sync.Once
}

// SeriesCatalog 在首次使用时惰性构造序列目录。
func (s *Server) SeriesCatalog() *store.SeriesCatalog {
	s.seriesCatalogOnce.Do(func() {
		s.seriesCatalog = s.store.NewSeriesCatalog(5 * time.Second)
	})
	return s.seriesCatalog
}

// New 返回一个新的 Server。
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
		quota:       NewQuotaTracker(),
		breaker:     NewBreakerRegistry(),
		webhooks:    NewWebhookDispatcher(),
		retention:   NewRetentionManager(),
		adminKeys:   auth.NewAdminStore(),
		replica:     NewReplicaStatus(),
		oidc:        NewOIDCRegistry(),
	}
}

// ServerConfig 汇总管理端点所需的配置。
type ServerConfig struct {
	DataDir string
}

// SetConfig 绑定运行时配置（数据目录等）。
func (s *Server) SetConfig(c ServerConfig) { s.cfg = c }

// Audit 返回审计日志，以便调用方在启动时配置容量，
// 或在测试时替换实现。
func (s *Server) Audit() *AuditLog { return s.auditLog }

// SetAuditLog 替换默认的审计日志。
// 在测试中或接入远程 sink 时很有用。
func (s *Server) SetAuditLog(l *AuditLog) { s.auditLog = l }

// Quota 返回按租户的配额跟踪器（第 42 轮）。
func (s *Server) Quota() *QuotaTracker { return s.quota }

// Breakers 返回熔断器注册表。
func (s *Server) Breakers() *BreakerRegistry { return s.breaker }

// Webhooks 返回 webhook 分发器句柄。
func (s *Server) Webhooks() *WebhookDispatcher { return s.webhooks }

// Retention 返回留存管理器句柄。
func (s *Server) Retention() *RetentionManager { return s.retention }

// AdminKeys 返回管理 API 密钥存储。
func (s *Server) AdminKeys() *auth.AdminStore { return s.adminKeys }

// Replica 返回副本状态句柄。
func (s *Server) Replica() *ReplicaStatus { return s.replica }

// OIDC 返回 OIDC 注册表。
func (s *Server) OIDC() *OIDCRegistry { return s.oidc }

// Datasources 暴露 datasource 注册表，以便调用方（例如启动时的
// driver 插件）注册额外后端。
func (s *Server) Datasources() *datasourceRegistry {
	return s.datasources
}

// Auth 暴露 API 密钥注册表，调用方可在启动时
//（或在测试中）注册密钥。AuthMode() 告诉中间件启用哪种
// 模式。
func (s *Server) Auth() *APIKeyAuth    { return s.auth }
func (s *Server) AuthMode() AuthMode    { return s.authM }
func (s *Server) SetAuthMode(m AuthMode) { s.authM = m }

// SetAllowedOrigins 将 CORS Access-Control-Allow-Origin 限制为
// 指定的主机列表。空列表保持通配符默认。
// 主机按精确字符串匹配（不含协议无关的怪异规则）；
// 若只想允许开发前端，可设置 http://localhost:3000。
func (s *Server) SetAllowedOrigins(origins []string) {
	s.allowedOrigins = origins
}

// SetRateLimit 安装一个按 IP 的令牌桶限流器。
// 传入 rate=0 即可禁用。
func (s *Server) SetRateLimit(rate, burst float64) {
	if rate <= 0 {
		s.rateLimiter = nil
		return
	}
	s.rateLimiter = NewRateLimiter(rate, burst)
}

// Handler 返回挂载了所有路由的根 http.Handler。
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
	// OTLP/HTTP 标准传输 (https://opentelemetry.io/docs/specs/otlp/#otlphttp)。
	// 每个信号都有自己的端点，以便按类型 fan out 的 collector / agent
	// 能够找到预期的路径。
	mux.HandleFunc("/v1/logs", s.handleOTLPHTTPLogs)
	mux.HandleFunc("/v1/metrics", s.handleOTLPHTTPMetrics)
	mux.HandleFunc("/v1/traces", s.handleOTLPHTTPTraces)
	mux.HandleFunc("/api/v1/series", s.handleSeries)
	mux.HandleFunc("/api/v1/metadata", s.handleMetadata)
	// Grafana / Alertmanager 的 PromQL 端点。PromQL 的子集：
	// 带 label filter 的 selector、sum/avg/count by (dim)、
	// rate(metric[1m])、histogram_quantile(q, metric)。
	mux.HandleFunc("/api/v1/query", s.handlePromQL)
	// Prometheus Remote Write 1.0 — 同时接受 /api/v1/write
	// (规范路径) 和 /api/prom/write (别名路径)。
	// 协议文档地址：
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
	mux.HandleFunc("/api/v1/rules", s.handleRules)
	mux.HandleFunc("/api/v1/rules/", s.handleRulesAdmin)
	mux.HandleFunc("/api/alerts/fires", s.handleAlertsFires)
	mux.HandleFunc("/api/tenants", s.handleTenantsDispatch)
	mux.HandleFunc("/api/tenants/", s.handleTenantsDispatch)
	mux.HandleFunc("/metrics", s.handlePromMetrics)

	// 第 53 轮对齐端点（admin_v1.go）。
	mux.HandleFunc("/api/v1/quota", s.handleQuota)
	mux.HandleFunc("/api/v1/slos", s.handleSLOs)
	mux.HandleFunc("/api/v1/slos/decide", s.handleSLODecide)
	mux.HandleFunc("/api/admin/keys", s.handleAdminKeys)
	mux.HandleFunc("/api/admin/keys/", s.handleAdminKeyItem)
	mux.HandleFunc("/api/v1/circuits", s.handleCircuits)
	mux.HandleFunc("/api/v1/circuits/", s.handleCircuitItem)
	mux.HandleFunc("/api/v1/ratelimits", s.handleRateLimits)
	mux.HandleFunc("/api/v1/webhooks", s.handleWebhooks)
	mux.HandleFunc("/api/v1/webhooks/", s.handleWebhookItem)
	mux.HandleFunc("/api/v1/webhooks/dlq", s.handleWebhookDLQ)
	mux.HandleFunc("/api/v1/webhooks/stats", s.handleWebhookStats)
	mux.HandleFunc("/api/v1/retention", s.handleRetention)
	mux.HandleFunc("/api/v1/retention/", s.handleRetentionReport)
	mux.HandleFunc("/api/v1/backups", s.handleBackups)
	mux.HandleFunc("/api/v1/backups/verify", s.handleBackupsVerify)
	mux.HandleFunc("/api/v1/backups/restore", s.handleBackupsRestore)
	mux.HandleFunc("/api/replica/state", s.handleReplicaState)
	mux.HandleFunc("/api/v1/auth/oidc", s.handleOIDC)
	mux.HandleFunc("/api/v1/auth/oidc/discovery", s.handleOIDCDiscovery)

	// 分层（由外向内）：
	//   withCORS -> 审计 -> rateLimit -> selfTrace -> latency ->
	//   (pprof + auth.Middleware) -> applyRoleGates -> mux
	//
	// auth.Middleware 在角色网关之前运行，以便它能在
	// 请求头中盖上 X-Dog-Role。pprof 路由挂在独立的层中，
	// 这样 /debug/pprof/* 请求永远不会进入 auth 网关。
	// 进入 auth 网关。
	gated := s.applyRoleGates(mux)
	h := s.auth.Middleware(s.authM,
		"/api/health", "/metrics", "/api/probe",
	)(gated)
	if s.pprofToken != "" {
		h = s.buildPProfMux(h)
	}
	if s.rateLimiter != nil {
	// 内层：按 API key 的桶 (在 auth 中间件解析 bearer token 之后)。
	// 外层：按 IP 的桶 (用于未认证的滥用防护)。
	// 两者都是短路求值，因此开销是有界的。
		h = s.rateLimiter.Middleware()(h)
		h = s.rateLimiter.KeyedMiddleware(func(r *http.Request) string {
			if k := r.Header.Get("Authorization"); k != "" {
				return k
			}
			return r.Header.Get("X-Dog-Tenant")
		})(h)
	}
	if s.auditLog != nil {
		h = AuditMiddleware(s.auditLog, false)(h)
	}
	h = s.selfTraceMiddleware(h)
	h = perHandlerLatency(h)
	return s.withCORS(withLogging(h))
}

// perHandlerLatency 将 http.Handler 包裹在一个直方图中，
// 记录每次请求的耗时（墙上时钟），
// 按 HTTP 方法和（粗略的）路由打标签。这里的 "route" 是去掉了查询串
// 和尾部服务标识的 URL 路径，这样即使存在多个不同的服务，
// 基数也能保持稳定。
//
// Exposed via /metrics under 名称 `dog_request_duration_seconds`.
func perHandlerLatency(next http.Handler) http.Handler {
	// 使用为可观测后端调优的固定桶边界集：
	// 1 ms ... 30 s。桶是全局的（非按路由），以保持指标基数有界。
	// 以保持指标基数有界。
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start).Seconds()
		route := trimRoute(r.URL.Path)
		requestDuration.WithLabelValues(r.Method, route).Observe(dur)
	})
}

// trimRoute 折叠噪杂的路径段以保持指标基数可预测：
// 类似 service id 的段被替换为 `{name}`，
// 类似 span id 的十六进制字符串被替换为 `{id}`。
// 其余保持原样。
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
			// 检测长度 >= 16 的纯十六进制片段（span / trace ID）。
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

// MountPProf 在给定前缀处注册 net/http/pprof 处理器，
// 通过 token query 参数进行门控。token 检查在任何 pprof 处理器之前运行，
// 因此仅仅 URL 泄漏是不够的。
func (s *Server) MountPProf(prefix, token string) {
	s.pprofPrefix = prefix
	s.pprofToken = token
}

// SetTenants 将 tenant 注册表连接到 server。一旦附加了注册表，
// /api/tenants 端点就生效了。
func (s *Server) SetTenants(reg *tenants.Registry) {
	s.tenants = reg
}

// applyRoleGates 返回一个按 role 对特定路由进行门控的处理器。
// 不在门控列表中的内容原样通过。
func (s *Server) applyRoleGates(next http.Handler) http.Handler {
	adminOnly := map[string]bool{
		"/api/audit":       true,
		"/api/audit/stats": true,
		"/api/keys":         true,
		"/api/seed":         true,
		"/api/seed/stream":  true,
		"/api/tenants":      true,
	}
	// writer+ 表示 ingest 对 writer（默认）和 admin 角色开放。
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
			// 未知的 origin — 不返回 ACAO 头部，
			// 让浏览器拒绝该响应。
				w.WriteHeader(http.StatusForbidden)
				return
			}
		} else if wildcard {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		// R3: 补齐 PATCH / DELETE / PUT。前端 tenants、admin keys、
		// alerts/rule、SLO/retention、backups 等多处
		// 用到这三个 method——原 allow-list 只有 GET/POST/OPTIONS
		// 会让跨域前端被浏览器 preflight 拒绝。
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Tenant-Id, traceparent")
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
	card := s.store.CardinalityStats()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"uptime":  time.Since(s.started).String(),
		"engine":  st,
		"cardinality": card,
		"version": "demo-dog-0.1.0",
		"now":     time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	// Tenant 过滤：优先使用 auth 绑定的 tenant (X-Dog-Tenant)，
	// 回退到 ?tenant=... (由平台管理员用于模拟)。
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
	// /api/services/{name}/detail -> 钻取详情负载（端点、错误、追踪）。
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

// WrapWithSelfTrace 启用自追踪。启用后，每一个
// 通过服务器的请求都会生成一个 OTLP span（POST 到
// 回环的 /api/ingest/otlp），让采集器可以绘制自身的延迟。
// trace ID 在此处生成；遵循 W3C tracecontext 的下游 SDK
// 会拼接到同一棵树中。
func (s *Server) WrapWithSelfTrace(loopback string) {
	selfTraceMu.Lock()
	selfTraceEnabled = true
	selfTraceLoopback = loopback
	selfTraceMu.Unlock()
}

// handleProbe 同时承担两种角色:
//
//   - K8s readinessProbe / 负载均衡器健康检查:
//     不带 ?target= 查询参数,返回 200 OK 与引擎统计;
//     不需要鉴权,以避免错误的 auth 配置导致采集器离线。
//   - 一次性外部 HTTP 探测(前端 Probes 页):
//     带 ?target=<url> 时,真正发起 HTTP 请求并返回
//     {ok, duration_ns, status_code, target}。
//     鉴权取决于角色门控(与查询端点相同)。
//     探测超时 5s。
func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		// R3: 把 stats dump 包装成 ProbeResult 兼容的形状,
		// 不然前端 probe() 即使没传 target 也会拿到一个
		// 缺 ok / duration_ns / status_code 的对象,UI 解析时会
		// 报 undefined。stats 字段全部平铺在顶层保留向后兼容。
		stats := s.store.Stats()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":               true,
			"target":           "self",
			"status_code":      http.StatusOK,
			"duration_ns":      int64(0),
			"uptime_seconds":   int(time.Since(s.started).Seconds()),
			"logs_accepted":    stats.LogsAccepted,
			"metrics_accepted": stats.MetricsAccepted,
			"spans_accepted":   stats.SpansAccepted,
			"queries_served":   stats.QueriesServed,
		})
		return
	}
	probeExternal(w, target)
}

// probeExternal 真正发起一次外部 HTTP GET 探测。
//
// 返回 200 OK 及 {ok, duration_ns, status_code, target},
// 与前端 ProbeResult 类型一一对应。
func probeExternal(w http.ResponseWriter, target string) {
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           false,
			"target":       target,
			"status_code":  0,
			"duration_ns":  int64(0),
		})
		return
	}
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          false,
			"target":      target,
			"status_code": 0,
			"duration_ns": int64(elapsed),
		})
		return
	}
	resp.Body.Close()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          resp.StatusCode >= 200 && resp.StatusCode < 400,
		"target":      target,
		"status_code": resp.StatusCode,
		"duration_ns": int64(elapsed),
	})
}

// buildPProfMux 将 `next` 包裹在一个小的 mux 中，
// 用于处理配置好的 /debug/pprof/* 路径（每个都
// 由配置的令牌保护），其他请求则透传到 next。
// 由 Handler() 在调用了 MountPProf 时调用。
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
