// tenants_test.go: 验证 /api/tenants/<id>/keys 系列端点的端到端行为。
//
// 这些测试以黑色盒子的方式对一个完整的 Server 进行真实 HTTP 调用,
// 确认 dispatcher、tenant registry 与 admin key store 协同工作。

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zsy619/demo-dog/backend/internal/xdata/tenants"
)

// newTenantsTestServer 返回一个启用了 tenants + adminKeys 的 Server。
// 使用 AuthModeOff 以避免每个请求都要求 API key。
func newTenantsTestServer(t *testing.T) *Server {
	t.Helper()
	s := newAdminTestServer(t)
	if s.adminKeys == nil {
		t.Skip("adminKeys not initialised")
	}
	s.SetTenants(tenants.New())
	return s
}

// TestTenantsLifecycle 测试完整的:create tenant -> mint key -> list keys
// -> rotate -> revoke 流程。
func TestTenantsLifecycle(t *testing.T) {
	s := newTenantsTestServer(t)

	// 1. 创建 tenant。
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tenants",
		strings.NewReader(`{"id":"acme","name":"ACME","description":"test"}`))
	s.handleTenantsDispatch(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create tenant: status=%d body=%s", w.Code, w.Body.String())
	}

	// 2. Mint key(POST)。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/tenants/acme/keys",
		strings.NewReader(`{"label":"checkout","role":"writer"}`))
	s.handleTenantsDispatch(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("mint key: status=%d body=%s", w.Code, w.Body.String())
	}

	// 3. List keys(GET) —— 现在必须返回至少 1 条。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tenants/acme/keys", nil)
	s.handleTenantsDispatch(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list keys: status=%d body=%s", w.Code, w.Body.String())
	}
	var listResp struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(),(&listResp)); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Keys) == 0 {
		t.Errorf("expected at least 1 key, got 0; body=%s", w.Body.String())
	}
	keyID, _ := listResp.Keys[0]["id"].(string)
	if keyID == "" {
		t.Errorf("key missing id field: %v", listResp.Keys[0])
	}

	// 4. Rotate(POST /rotate)。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/tenants/acme/keys/"+keyID+"/rotate", nil)
	s.handleTenantsDispatch(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rotate: status=%d body=%s", w.Code, w.Body.String())
	}
	var rotateResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(),(&rotateResp)); err != nil {
		t.Fatalf("decode rotate: %v", err)
	}
	newID, _ := rotateResp["key_id"].(string)
	if newID == "" || newID == keyID {
		t.Errorf("rotate returned bad key_id: %q (old=%q)", newID, keyID)
	}

	// 5. Revoke(DELETE)。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/tenants/acme/keys/"+newID, nil)
	s.handleTenantsDispatch(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke: status=%d body=%s", w.Code, w.Body.String())
	}

	// 6. 列表应该不再包含被撤销的 key。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tenants/acme/keys", nil)
	s.handleTenantsDispatch(w, req)
	var afterRevoke struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(),(&afterRevoke)); err != nil {
		t.Fatalf("decode list2: %v", err)
	}
	for _, k := range afterRevoke.Keys {
		if id, _ := k["id"].(string); id == newID {
			t.Errorf("revoked key %s still listed", newID)
		}
	}
}

// TestTenantKeysUnknownTenant 404 当 tenant 不存在时。
func TestTenantKeysUnknownTenant(t *testing.T) {
	s := newTenantsTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tenants/ghost/keys", nil)
	s.handleTenantsDispatch(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
