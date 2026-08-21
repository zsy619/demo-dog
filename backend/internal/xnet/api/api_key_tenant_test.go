package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// W2.1: 多租户 API-key 隔离

func TestAPIKey_EnsureTenantAccess_SameTenantAllowed(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddForTenant("key-1", "alice", "acme", RoleWriter)
	if err := a.EnsureTenantAccess("key-1", "acme"); err != nil {
		t.Fatalf("same-tenant must pass, got %v", err)
	}
}

func TestAPIKey_EnsureTenantAccess_CrossTenantRejected(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddForTenant("key-1", "alice", "acme", RoleWriter)
	err := a.EnsureTenantAccess("key-1", "globex")
	if !IsTenantMismatch(err) {
		t.Fatalf("cross-tenant must reject with ErrTenantMismatch, got %v", err)
	}
}

func TestAPIKey_EnsureTenantAccess_EmptyClaimedByWriterRejected(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddForTenant("key-1", "alice", "acme", RoleWriter)
	if err := a.EnsureTenantAccess("key-1", ""); !IsTenantMismatch(err) {
		t.Fatalf("writer with empty claimed must reject, got %v", err)
	}
}

func TestAPIKey_EnsureTenantAccess_PlatformKeyBypassesTenant(t *testing.T) {
	a := NewAPIKeyAuth()
	a.Add("platform-key", "ops", RoleAdmin) // tenant empty = platform admin
	if err := a.EnsureTenantAccess("platform-key", "anything"); err != nil {
		t.Fatalf("admin key must bypass tenant check, got %v", err)
	}
}

func TestAPIKey_EnsureTenantAccess_AdminRoleBypassesTenant(t *testing.T) {
	a := NewAPIKeyAuth()
	a.Add("ops", "ops", RoleAdmin) // tenant empty, admin role
	if err := a.EnsureTenantAccess("ops", "globex"); err != nil {
		t.Fatalf("admin-role key must bypass tenant check, got %v", err)
	}
}

func TestAPIKey_EnsureTenantAccess_UnknownKeyError(t *testing.T) {
	a := NewAPIKeyAuth()
	err := a.EnsureTenantAccess("nope", "acme")
	if IsTenantMismatch(err) {
		t.Fatalf("unknown key must NOT be ErrTenantMismatch (that's misleading), got %v", err)
	}
}

// 跨租户中间件测试: 客户端 X-Tenant-Id 或 ?tenant= 与 key 绑定不符时,
// 必须返回 403 而不是悄悄放行后用 X-Dog-Tenant 覆盖。

func TestMiddleware_CrossTenantHeader_Rejected(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddForTenant("key-1", "alice", "acme", RoleWriter)

	h := a.Middleware(AuthModeAPIKey, "/public")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/services", nil)
	req.Header.Set("Authorization", "Bearer key-1")
	req.Header.Set("X-Tenant-Id", "globex") // cross-tenant attempt
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant header must 403, got %d", rr.Code)
	}
}

func TestMiddleware_CrossTenantQuery_Rejected(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddForTenant("key-1", "alice", "acme", RoleWriter)

	h := a.Middleware(AuthModeAPIKey)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/services?tenant=globex", nil)
	req.Header.Set("Authorization", "Bearer key-1")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant query must 403, got %d", rr.Code)
	}
}

func TestMiddleware_SameTenantHeader_AllowedAndOverwritten(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddForTenant("key-1", "alice", "acme", RoleWriter)

	var seenTenant string
	h := a.Middleware(AuthModeAPIKey)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seenTenant = r.Header.Get("X-Tenant-Id")
			w.WriteHeader(http.StatusOK)
		}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/services?tenant=acme", nil)
	req.Header.Set("Authorization", "Bearer key-1")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("matching tenant must pass, got %d", rr.Code)
	}
	if seenTenant != "acme" {
		t.Errorf("downstream must see bound tenant, got %q", seenTenant)
	}
}

func TestMiddleware_AdminKeyBypassesTenantEnforcement(t *testing.T) {
	a := NewAPIKeyAuth()
	a.Add("ops", "ops", RoleAdmin)

	h := a.Middleware(AuthModeAPIKey)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/services?tenant=anything", nil)
	req.Header.Set("Authorization", "Bearer ops")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin must bypass cross-tenant check, got %d", rr.Code)
	}
}

func TestMiddleware_NoClaimedTenant_AllowedForBoundKey(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddForTenant("key-1", "alice", "acme", RoleWriter)

	h := a.Middleware(AuthModeAPIKey)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/services", nil)
	req.Header.Set("Authorization", "Bearer key-1")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bound key without explicit claim must pass, got %d", rr.Code)
	}
}
