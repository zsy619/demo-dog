package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
	"github.com/zsy619/demo-dog/backend/internal/xdata/retention"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
	"github.com/zsy619/demo-dog/backend/internal/xnet/webhook"
)

func newAdminTestServer(t *testing.T) *Server {
	t.Helper()
	d := store.New(store.DefaultConfig())
	s := New(d, nil, nil)
	s.SetAuthMode(AuthModeOff)
	s.SetConfig(ServerConfig{DataDir: t.TempDir()})
	return s
}

func TestHandleQuota_Empty(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/quota", nil)
	s.handleQuota(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleQuota_TenantMissing(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/quota?tenant=acme", nil)
	s.handleQuota(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleSLOs_Empty(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slos", nil)
	s.handleSLOs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleSLOs_WithSLO(t *testing.T) {
	s := newAdminTestServer(t)
	s.alerts.AddSLO(&alerts.SLO{
		Name:         "checkout",
		Service:      "checkout",
		Target:       0.99,
		Window:       time.Hour,
		TotalCounter: "checkout_total",
		BadCounter:   "checkout_bad",
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slos", nil)
	s.handleSLOs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var body struct {
		SLOs []map[string]any `json:"slos"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.SLOs) != 1 || body.SLOs[0]["name"] != "checkout" {
		t.Fatalf("unexpected: %+v", body)
	}
}

func TestHandleSLODecide(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slos/decide?short_ns=300000000&long_ns=3600000000000", nil)
	s.handleSLODecide(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleSLODecide_Bad(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slos/decide", nil)
	s.handleSLODecide(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleAdminKeys_Get(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/keys", nil)
	s.handleAdminKeys(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleAdminKeys_Post(t *testing.T) {
	s := newAdminTestServer(t)
	body := bytes.NewReader([]byte(`{"label":"ops","tenant":"acme","role":"admin","scopes":["audit:read"]}`))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/keys", body)
	s.handleAdminKeys(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleAdminKeys_BadMethod(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/keys", nil)
	s.handleAdminKeys(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleCircuits(t *testing.T) {
	s := newAdminTestServer(t)
	s.Breakers().Get("ingest")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/circuits", nil)
	s.handleCircuits(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleCircuitItem_Reset(t *testing.T) {
	s := newAdminTestServer(t)
	s.Breakers().Get("ingest")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/circuits/ingest/reset", nil)
	s.handleCircuitItem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleRateLimits(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ratelimits", nil)
	s.handleRateLimits(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleWebhooks(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
	s.handleWebhooks(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

// TestHandleWebhookItem_Test 验证 POST /api/v1/webhooks/{id}/test。
//
// 使用一个本地 httptest 服务器作为订阅目标,断言响应
// 包含 status/latency_ns/event_id 等字段。
func TestHandleWebhookItem_Test(t *testing.T) {
	s := newAdminTestServer(t)
	d := s.webhooks.Dispatcher()
	if d == nil {
		t.Skip("webhook dispatcher not initialised")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer target.Close()
	if err := d.AddSubscriber(&webhook.Subscriber{
		ID: "probe", URL: target.URL, Secret: "k", EventTypes: []string{"*"},
	}); err != nil {
		t.Fatal(err)
	}
	defer d.RemoveSubscriber("probe")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/probe/test",
		strings.NewReader(`{"type":"manual","payload":{"k":"v"},"tenant":"acme"}`))
	s.handleWebhookItem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(),(&out)); err != nil {
		t.Fatal(err)
	}
	if int(out["status"].(float64)) != 200 {
		t.Errorf("status=%v", out["status"])
	}
	if out["subscriber_id"] != "probe" {
		t.Errorf("subscriber_id=%v", out["subscriber_id"])
	}
	if _, ok := out["latency_ns"]; !ok {
		t.Errorf("missing latency_ns: %v", out)
	}
}

// TestHandleWebhookItem_Test_NotFound 验证不存在的订阅者返回 404。
func TestHandleWebhookItem_Test_NotFound(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ghost/test",
		strings.NewReader(`{}`))
	s.handleWebhookItem(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleRetention_Get(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/retention", nil)
	s.handleRetention(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleRetention_Put(t *testing.T) {
	s := newAdminTestServer(t)
	body := bytes.NewReader([]byte(`{"tenant":"acme","tier":"pro","hot_ttl_ns":3600000000000,"cold_ttl_ns":86400000000000}`))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/retention", body)
	s.handleRetention(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleRetentionReport(t *testing.T) {
	s := newAdminTestServer(t)
	m := s.Retention().Manager()
	_ = m.SetPolicy(retention.Policy{Tenant: "acme", Tier: retention.TierFree})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/retention/acme/report", nil)
	s.handleRetentionReport(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleBackups(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", nil)
	s.handleBackups(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleReplicaState(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/replica/state", nil)
	s.handleReplicaState(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleOIDC_Get(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc", nil)
	s.handleOIDC(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandleOIDC_PutDelete(t *testing.T) {
	s := newAdminTestServer(t)
	body := bytes.NewReader([]byte(`{"issuer":"https://idp.example.com","client_id":"abc","audiences":["dog"],"scopes":["openid"]}`))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/oidc", body)
	s.handleOIDC(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status: %d body=%s", rr.Code, rr.Body.String())
	}
	delRR := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oidc?issuer=https://idp.example.com", nil)
	s.handleOIDC(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete status: %d", delRR.Code)
	}
}

func TestHandleOIDC_DeleteBad(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oidc", nil)
	s.handleOIDC(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rr.Code)
	}
}

// TestHandleOIDC_PutGroupsClaim 验证 PUT 时前端发送的 groups_claim
// 会被持久化,而不是被硬编码覆盖回 "groups" (R3 修复)。
func TestHandleOIDC_PutGroupsClaim(t *testing.T) {
	s := newAdminTestServer(t)
	body := bytes.NewReader([]byte(`{"issuer":"https://idp.example.com","client_id":"abc","audiences":["dog"],"scopes":["openid"],"groups_claim":"departments"}`))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/oidc", body)
	s.handleOIDC(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status: %d", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["groups_claim"] != "departments" {
		t.Errorf("groups_claim not round-tripped: %v", got["groups_claim"])
	}
	// 后续 GET 应回带自定义的 groups_claim 而不是默认 "groups"。
	listRR := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc", nil)
	s.handleOIDC(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("get status: %d", listRR.Code)
	}
	var listResp struct {
		Providers []map[string]any `json:"providers"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, p := range listResp.Providers {
		if p["issuer"] == "https://idp.example.com" {
			if p["groups_claim"] != "departments" {
				t.Errorf("list groups_claim mismatch: %v", p["groups_claim"])
			}
			found = true
		}
	}
	if !found {
		t.Errorf("provider not in list")
	}
	// 清理。
	delRR := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oidc?issuer=https://idp.example.com", nil)
	s.handleOIDC(delRR, delReq)
}

// TestHandleOIDC_GroupsClaimFallback 验证旧客户端 PUT 时
// 不带 groups_claim 时回退到默认 "groups",保持兼容。
func TestHandleOIDC_GroupsClaimFallback(t *testing.T) {
	s := newAdminTestServer(t)
	body := bytes.NewReader([]byte(`{"issuer":"https://idp-old.example.com","client_id":"old"}`))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/oidc", body)
	s.handleOIDC(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status: %d", rr.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got["groups_claim"] != "groups" {
		t.Errorf("expected fallback groups, got %v", got["groups_claim"])
	}
	// 清理。
	delRR := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oidc?issuer=https://idp-old.example.com", nil)
	s.handleOIDC(delRR, delReq)
}

// TestHandleProbe_NoTarget 验证 R3 后不带 target 的
// /api/probe 也返回 ProbeResult 兼容形状(ok/target/status_code/duration_ns)。
func TestHandleProbe_NoTarget(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/probe", nil)
	s.handleProbe(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"ok", "target", "status_code", "duration_ns"} {
		if _, ok := got[field]; !ok {
			t.Errorf("missing field %q in response: %v", field, got)
		}
	}
	if got["ok"] != true {
		t.Errorf("expected ok=true, got %v", got["ok"])
	}
}

// TestHandleProbe_WithTarget 验证带 target 的 /api/probe 走外部探测,
// 返回完整 ProbeResult。
func TestHandleProbe_WithTarget(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer target.Close()
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/probe?target="+target.URL, nil)
	s.handleProbe(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if int(got["status_code"].(float64)) != 204 {
		t.Errorf("expected status_code=204, got %v", got["status_code"])
	}
	if got["ok"] != true {
		t.Errorf("expected ok=true, got %v", got["ok"])
	}
}

// TestHandleBackups_POST 验证 R4 修复:前端 createBackup
// 一直 POST /api/v1/backups,旧 handler 只接受 GET,R4
// 之后必须能成功创建备份并返回 BackupResult 视图。
func TestHandleBackups_POST(t *testing.T) {
	s := newAdminTestServer(t)
	output := t.TempDir() + "/backup.tar"
	body := strings.NewReader(`{"output":"` + output + `","compress":false}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups", body)
	s.handleBackups(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["output"] != output {
		t.Errorf("output mismatch: %v", got["output"])
	}
	if got["sha256"] == "" {
		t.Errorf("missing sha256: %v", got["sha256"])
	}
	if got["snapshot_id"] == "" {
		t.Errorf("missing snapshot_id: %v", got["snapshot_id"])
	}
	if got["taken_at"] == "" {
		t.Errorf("missing taken_at: %v", got["taken_at"])
	}
}

// TestHandleBackups_POST_MissingOutput 验证缺少 output 字段时返回 400。
func TestHandleBackups_POST_MissingOutput(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups",
		strings.NewReader(`{"compress":false}`))
	s.handleBackups(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestHandleBackups_MethodNotAllowed 验证 PUT/PATCH/DELETE 仍
// 返回 405,避免 R4 之后的 POST 路径意外放开其它 method。
func TestHandleBackups_MethodNotAllowed(t *testing.T) {
	s := newAdminTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/backups", nil)
	s.handleBackups(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Allow") != "GET POST" {
		t.Errorf("Allow header wrong: %q", rr.Header().Get("Allow"))
	}
}
