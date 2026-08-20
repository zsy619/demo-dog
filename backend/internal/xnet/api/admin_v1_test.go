package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
	"github.com/zsy619/demo-dog/backend/internal/xdata/retention"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
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
