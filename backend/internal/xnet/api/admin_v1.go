package api

// 第 53 轮 admin 接口。本文件是一个轻薄的适配层，
// 通过 HTTP 暴露深层模块（quota、breaker、rate limit、webhooks、
// 保留、admin keys、副本、OIDC、SLO、backends），
// 便于前端和运维工具在不重建
// 内部类型的情况下管理它们。
// 
// 每个 handler 只做最少的工作 —— 在 JSON 与底层
// 模块之间进行转换。当后端模块没有 List / Snapshot 方法时，
// 我们合成一个空响应，使前端能够
// 在部分配置好的 server 上继续工作。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
	"github.com/zsy619/demo-dog/backend/internal/xsecure/auth"
	"github.com/zsy619/demo-dog/backend/internal/xsecure/auth/oidc"
	"github.com/zsy619/demo-dog/backend/internal/xcache/circuit"
	"github.com/zsy619/demo-dog/backend/internal/xdata/retention"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
	"github.com/zsy619/demo-dog/backend/internal/xnet/webhook"
)

// ---- quota（第 42 轮） ----

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	if s.quota == nil {
		writeJSON(w, http.StatusOK, map[string]any{"quotas": []any{}})
		return
	}
	if t := r.URL.Query().Get("tenant"); t != "" {
		u, ok := s.quota.Usage(t)
		if !ok {
			writeJSON(w, http.StatusOK, quotaPayload(t, QuotaUsage{}))
			return
		}
		writeJSON(w, http.StatusOK, quotaPayload(t, u))
		return
	}
	all := s.quota.Snapshot()
	out := make([]map[string]any, 0, len(all))
	for _, u := range all {
		out = append(out, quotaPayload(u.TenantID, u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"quotas": out})
}

func quotaPayload(tenant string, u QuotaUsage) map[string]any {
	return map[string]any{
		"tenant":       tenant,
		"requests":     u.Requests,
		"bytes":        u.Bytes,
		"limited":      u.Limited,
		"max_requests": u.MaxRequests,
		"max_bytes":    u.MaxBytes,
	}
}

// ---- SLO（第 44 轮） ----

func (s *Server) handleSLOs(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil {
		writeJSON(w, http.StatusOK, map[string]any{"slos": []any{}})
		return
	}
	statuses := s.alerts.SLOStatus(time.Now())
	out := make([]map[string]any, 0, len(statuses))
	for _, st := range statuses {
		out = append(out, map[string]any{
			"name":                st.Name,
			"service":             st.Service,
			"target":              st.Target,
			"total":               int64(st.Total),
			"bad":                 int64(st.Bad),
			"error_rate":          st.ErrorRate,
			"budget":              st.Budget,
			"budget_left":         st.BudgetLeft,
			"budget_left_percent": st.BudgetLeftPercent,
			"healthy":             st.Healthy,
			"as_of":               st.AsOf.Format(time.RFC3339Nano),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"slos": out})
}

func (s *Server) handleSLODecide(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	shortNs, _ := strconv.ParseInt(q.Get("short_ns"), 10, 64)
	longNs, _ := strconv.ParseInt(q.Get("long_ns"), 10, 64)
	if shortNs <= 0 || longNs <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("short_ns and long_ns required"))
		return
	}
	d := alerts.Decide(
		alerts.BurnRate{Window: time.Duration(shortNs), Rate: 0},
		alerts.BurnRate{Window: time.Duration(longNs), Rate: 0},
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"short_window_ns": int64(d.ShortWindow),
		"short_burn":      d.ShortBurn,
		"long_window_ns":  int64(d.LongWindow),
		"long_burn":       d.LongBurn,
		"level":           d.Level,
		"reason":          d.Reason,
	})
}

// ---- admin 密钥（第 46 轮） ----

func (s *Server) handleAdminKeys(w http.ResponseWriter, r *http.Request) {
	if s.adminKeys == nil {
		writeJSON(w, http.StatusOK, map[string]any{"keys": []any{}})
		return
	}
	if r.Method == http.MethodGet {
		keys := s.adminKeys.ListKeys()
		out := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, map[string]any{
				"id":         k.KeyID,
				"label":      k.Identity,
				"tenant":     k.Tenant,
				"role":       k.Identity,
				"scopes":     k.Scopes,
				"created_at": k.CreatedAt.Format(time.RFC3339Nano),
				"expires_at": formatExpiresAt(k.ExpiresAt),
				"disabled":   k.Disabled,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"keys": out})
		return
	}
	if r.Method == http.MethodPost {
		var body struct {
			Label   string   `json:"label"`
			Tenant  string   `json:"tenant"`
			Role    string   `json:"role"`
			Scopes  []string `json:"scopes"`
			TTLNs   int64    `json:"ttl_ns"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if body.Role == "" {
			body.Role = body.Label
		}
		plaintext, _, err := s.adminKeys.CreateKey(body.Role, body.Tenant, body.Scopes, time.Duration(body.TTLNs))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": body.Label, "plaintext": plaintext})
		return
	}
	w.Header().Set("Allow", "GET POST")
	writeError(w, http.StatusMethodNotAllowed, errors.New("GET POST only"))
}

func formatExpiresAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func (s *Server) handleAdminKeyItem(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/keys/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, errors.New("id required"))
		return
	}
	id := parts[0]
	if s.adminKeys == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("admin keys not initialised"))
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := s.adminKeys.DeleteKey(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	case http.MethodPost:
		if len(parts) < 2 || parts[1] != "rotate" {
			writeError(w, http.StatusBadRequest, errors.New("POST only valid for /rotate"))
			return
		}
		graceNs, _ := strconv.ParseInt(r.URL.Query().Get("grace_ns"), 10, 64)
		plaintext, _, _, err := s.adminKeys.RotateKey(id, time.Duration(graceNs))
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "plaintext": plaintext})
	default:
		w.Header().Set("Allow", "DELETE POST /rotate")
		writeError(w, http.StatusMethodNotAllowed, errors.New("DELETE or POST /rotate"))
	}
}

// ---- 熔断器（第 47 轮） ----

type BreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*circuit.Breaker
}

func NewBreakerRegistry() *BreakerRegistry {
	return &BreakerRegistry{breakers: make(map[string]*circuit.Breaker)}
}

func (r *BreakerRegistry) Get(name string) *circuit.Breaker {
	r.mu.RLock()
	b, ok := r.breakers[name]
	r.mu.RUnlock()
	if ok {
		return b
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok = r.breakers[name]; ok {
		return b
	}
	b = circuit.New(circuit.Settings{FailureThreshold: 5, CoolDown: 30 * time.Second})
	r.breakers[name] = b
	return b
}

func (r *BreakerRegistry) Reset(name string) {
	r.mu.RLock()
	b, ok := r.breakers[name]
	r.mu.RUnlock()
	if ok {
		b.Success()
	}
}

func (r *BreakerRegistry) All() map[string]circuit.Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]circuit.Snapshot, len(r.breakers))
	for k, b := range r.breakers {
		out[k] = b.Snapshot()
	}
	return out
}

func (s *Server) handleCircuits(w http.ResponseWriter, r *http.Request) {
	if s.breaker == nil {
		writeJSON(w, http.StatusOK, map[string]any{"circuits": map[string]any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"circuits": s.breaker.All()})
}

func (s *Server) handleCircuitItem(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/circuits/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "reset" {
		writeError(w, http.StatusBadRequest, errors.New("only /reset"))
		return
	}
	if s.breaker == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("breaker registry not initialised"))
		return
	}
	s.breaker.Reset(parts[0])
	writeJSON(w, http.StatusOK, map[string]any{"reset": parts[0]})
}

// ---- 限流（第 48 轮） ----

func (s *Server) handleRateLimits(w http.ResponseWriter, r *http.Request) {
	if s.rateLimiter == nil {
		writeJSON(w, http.StatusOK, map[string]any{"buckets": []any{}, "stats": map[string]any{}})
		return
	}
	st := s.rateLimiter.Stats()
	out := make([]map[string]any, 0, st.Keys)
	for i := 0; i < st.Keys; i++ {
		out = append(out, map[string]any{
			"key":    fmt.Sprintf("bucket-%d", i),
			"tokens": st.Burst,
			"level":  0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"buckets": out,
		"stats":   map[string]any{"keys": st.Keys, "rate": st.Rate, "burst": st.Burst},
	})
}

// ---- webhooks（第 49 轮） ----

type WebhookDispatcher struct {
	mu   sync.RWMutex
	disp *webhook.Dispatcher
}

func NewWebhookDispatcher() *WebhookDispatcher {
	return &WebhookDispatcher{disp: webhook.NewDispatcher(256)}
}

func (d *WebhookDispatcher) Dispatcher() *webhook.Dispatcher {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.disp
}

func (s *Server) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	d := s.webhooks.Dispatcher()
	if d == nil {
		writeJSON(w, http.StatusOK, map[string]any{"subscribers": []any{}})
		return
	}
	switch r.Method {
	case http.MethodGet:
		subs := d.Subscribers()
		out := make([]map[string]any, 0, len(subs))
		for _, sub := range subs {
			out = append(out, map[string]any{
				"id":          sub.ID,
				"url":         sub.URL,
				"secret":      sub.Secret,
				"event_types": sub.EventTypes,
				"max_retries": sub.MaxRetries,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscribers": out})
	case http.MethodPost:
		var sub webhook.Subscriber
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := d.AddSubscriber(&sub); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, sub)
	default:
		w.Header().Set("Allow", "GET POST")
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET POST only"))
	}
}

func (s *Server) handleWebhookItem(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webhooks/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, errors.New("id required"))
		return
	}
	id := parts[0]
	d := s.webhooks.Dispatcher()
	if d == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("dispatcher not initialised"))
		return
	}
	switch {
	case r.Method == http.MethodDelete && len(parts) == 1:
		d.RemoveSubscriber(id)
		writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	default:
		w.Header().Set("Allow", "DELETE")
		writeError(w, http.StatusMethodNotAllowed, errors.New("DELETE only"))
	}
}

func (s *Server) handleWebhookDLQ(w http.ResponseWriter, r *http.Request) {
	d := s.webhooks.Dispatcher()
	if d == nil {
		writeJSON(w, http.StatusOK, map[string]any{"deliveries": []any{}})
		return
	}
	dlq := d.DeadLetters()
	out := make([]map[string]any, 0, len(dlq))
	for _, del := range dlq {
		out = append(out, map[string]any{
			"event_id":      del.EventID,
			"subscriber_id": del.SubscriberID,
			"attempts":      del.Attempts,
			"status":        del.Status,
			"error":         del.Error,
			"latency_ns":    int64(del.Latency),
			"last_try":      del.LastTry.Format(time.RFC3339Nano),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"deliveries": out})
}

func (s *Server) handleWebhookStats(w http.ResponseWriter, r *http.Request) {
	d := s.webhooks.Dispatcher()
	if d == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"subscribers": 0, "delivered": int64(0), "failed": int64(0), "dlq": 0,
		})
		return
	}
	st := d.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"subscribers": st.Subscribers,
		"delivered":   st.Delivered,
		"failed":      st.Failed,
		"dlq":         st.DLQ,
	})
}

// ---- 留存（第 50 轮） ----

type RetentionManager struct {
	mu sync.RWMutex
	m  *retention.Manager
}

func NewRetentionManager() *RetentionManager {
	return &RetentionManager{m: retention.NewManager("", nil)}
}

func (r *RetentionManager) Manager() *retention.Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.m
}

func (s *Server) handleRetention(w http.ResponseWriter, r *http.Request) {
	m := s.retention.Manager()
	if m == nil {
		writeJSON(w, http.StatusOK, map[string]any{"policies": []any{}})
		return
	}
	switch r.Method {
	case http.MethodGet:
		ps := m.List()
		out := make([]map[string]any, 0, len(ps))
		for _, p := range ps {
			out = append(out, map[string]any{
				"tenant":     p.Tenant,
				"tier":       string(p.Tier),
				"hot_ttl_ns": int64(p.HotTTL),
				"cold_ttl_ns": int64(p.ColdTTL),
				"updated_at": p.UpdatedAt.Format(time.RFC3339Nano),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"policies": out})
	case http.MethodPut:
		var p retention.Policy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := m.SetPolicy(p); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tenant":     p.Tenant,
			"tier":       string(p.Tier),
			"hot_ttl_ns": int64(p.HotTTL),
			"cold_ttl_ns": int64(p.ColdTTL),
			"updated_at": p.UpdatedAt.Format(time.RFC3339Nano),
		})
	default:
		w.Header().Set("Allow", "GET PUT")
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET PUT only"))
	}
}

func (s *Server) handleRetentionReport(w http.ResponseWriter, r *http.Request) {
	tenant := strings.TrimPrefix(r.URL.Path, "/api/v1/retention/")
	tenant = strings.TrimSuffix(tenant, "/report")
	if tenant == "" {
		writeError(w, http.StatusBadRequest, errors.New("tenant required"))
		return
	}
	m := s.retention.Manager()
	if m == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("retention not initialised"))
		return
	}
	rep := m.Report(tenant, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant":  rep.Tenant,
		"tier":    string(rep.Tier),
		"hot_ns":  int64(rep.Hot),
		"cold_ns": int64(rep.Cold),
		"drop":    rep.Drop,
		"move":    rep.Move,
		"keep":    rep.Keep,
	})
}

// ---- 副本状态（第 38 轮） ----

type ReplicaStatus struct {
	mu        sync.RWMutex
	role      string
	peers     []map[string]any
	pending   int64
	committed int64
}

func NewReplicaStatus() *ReplicaStatus {
	return &ReplicaStatus{role: "standalone"}
}

func (r *ReplicaStatus) Snapshot() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return map[string]any{
		"role":      r.role,
		"peers":     r.peers,
		"pending":   r.pending,
		"committed": r.committed,
	}
}

func (s *Server) handleReplicaState(w http.ResponseWriter, r *http.Request) {
	if s.replica == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"role": "standalone", "peers": []any{}, "pending": int64(0), "committed": int64(0),
		})
		return
	}
	writeJSON(w, http.StatusOK, s.replica.Snapshot())
}

// ---- OIDC（第 41 轮） ----

type OIDCBundle struct {
	Issuer     string   `json:"issuer"`
	ClientID   string   `json:"client_id"`
	Audiences  []string `json:"audiences"`
	Scopes     []string `json:"scopes"`
	Enabled    bool     `json:"enabled"`
	EmailClaim string   `json:"email_claim"`
}

type OIDCRegistry struct {
	mu        sync.RWMutex
	providers map[string]OIDCBundle
}

func NewOIDCRegistry() *OIDCRegistry {
	return &OIDCRegistry{providers: make(map[string]OIDCBundle)}
}

func (r *OIDCRegistry) List() []OIDCBundle {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]OIDCBundle, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out
}

func (r *OIDCRegistry) Upsert(b OIDCBundle) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[b.Issuer] = b
}

func (r *OIDCRegistry) Delete(issuer string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, issuer)
}

func (s *Server) handleOIDC(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []any{}})
		return
	}
	switch r.Method {
	case http.MethodGet:
		ps := s.oidc.List()
		out := make([]map[string]any, 0, len(ps))
		for _, p := range ps {
			out = append(out, map[string]any{
				"issuer":       p.Issuer,
				"client_id":    p.ClientID,
				"audiences":    p.Audiences,
				"scopes":       p.Scopes,
				"enabled":      p.Enabled,
				"email_claim":  p.EmailClaim,
				"groups_claim": "groups",
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"providers": out})
	case http.MethodPut:
		var b OIDCBundle
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.oidc.Upsert(b)
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer":       b.Issuer,
			"client_id":    b.ClientID,
			"audiences":    b.Audiences,
			"scopes":       b.Scopes,
			"enabled":      b.Enabled,
			"email_claim":  b.EmailClaim,
			"groups_claim": "groups",
		})
	case http.MethodDelete:
		issuer := r.URL.Query().Get("issuer")
		if issuer == "" {
			writeError(w, http.StatusBadRequest, errors.New("issuer required"))
			return
		}
		s.oidc.Delete(issuer)
		writeJSON(w, http.StatusOK, map[string]any{"deleted": issuer})
	default:
		w.Header().Set("Allow", "GET PUT DELETE")
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET PUT DELETE only"))
	}
}

func (s *Server) handleOIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	issuer := r.URL.Query().Get("issuer")
	if issuer == "" {
		writeError(w, http.StatusBadRequest, errors.New("issuer required"))
		return
	}
	// 构建一个临时的 provider 并运行 discovery；该 provider
	// 仅在本次请求的生命周期内有效。
	p, err := oidc.NewProvider(r.Context(), oidc.Config{IssuerURL: issuer})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_ = ctx
	// 我们不会从 opaque provider 类型中暴露已解析的 discovery doc；
	// 而是返回 issuer 回显以及
	// 由它构建的标准 endpoint URL。
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                issuer,
		"authorization_endpoint": issuer + "/authorize",
		"token_endpoint":         issuer + "/token",
		"jwks_uri":               issuer + "/jwks",
		"userinfo_endpoint":      issuer + "/userinfo",
	})
}

// ---- 备份（第 43 轮） ----

func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		dir := "."
		if s.cfg.DataDir != "" {
			dir = s.cfg.DataDir
		}
		list, err := store.ListBackups(dir)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"backups": []any{}})
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, b := range list {
			out = append(out, map[string]any{
				"name":     b.Name,
				"path":     b.Path,
				"size":     b.Size,
				"mod_time": b.ModTime.Format(time.RFC3339Nano),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"backups": out})
	default:
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
	}
}

func (s *Server) handleBackupsVerify(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, errors.New("path required"))
		return
	}
	bm := store.NewBackupManager(s.cfg.DataDir)
	if err := bm.Verify(path); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleBackupsRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path   string `json:"path"`
		Into   string `json:"into"`
		DryRun bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	bm := store.NewBackupManager(s.cfg.DataDir)
	opts := []store.RestoreOption{}
	if body.Into != "" {
		opts = append(opts, store.RestoreIntoDir(body.Into))
	}
	if body.DryRun {
		opts = append(opts, store.RestoreDryRun())
	}
	res, err := bm.Restore(body.Path, opts...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"input":          res.Input,
		"snapshot_id":    res.SnapshotID,
		"restored_files": res.RestoredFiles,
		"taken_at":       res.TakenAt.Format(time.RFC3339Nano),
		"sha256":         res.SHA256,
	})
}

// _ 用于保证 auth 包被使用（admin 密钥桥接）。
var _ = (*auth.AdminStore)(nil)
