// alerts_test.go: 验证 alerts 端点 (rules / fires) 的 method 门控与基本响应。

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleAlertsRules_GetOnly 验证 POST/PUT/DELETE 返回 405。
func TestHandleAlertsRules_GetOnly(t *testing.T) {
	s := newAdminTestServer(t)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/alerts/rules", nil)
		s.handleAlertsRules(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for %s, got %d", m, w.Code)
		}
	}
}

// TestHandleAlertsRules_Get 验证 GET 正常返回空数组。
func TestHandleAlertsRules_Get(t *testing.T) {
	s := newAdminTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/alerts/rules", nil)
	s.handleAlertsRules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

// TestHandleAlertsFires_GetOnly 验证非 GET 返回 405。
func TestHandleAlertsFires_GetOnly(t *testing.T) {
	s := newAdminTestServer(t)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/alerts/fires", nil)
		s.handleAlertsFires(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for %s, got %d", m, w.Code)
		}
	}
}

// TestHandleCircuitItem_PostOnly 验证非 POST 返回 405。
func TestHandleCircuitItem_PostOnly(t *testing.T) {
	s := newAdminTestServer(t)
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/v1/circuits/foo/reset", nil)
		s.handleCircuitItem(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for %s, got %d", m, w.Code)
		}
	}
}

// TestHandleBackupsVerify_PostOnly 验证非 POST 返回 405。
func TestHandleBackupsVerify_PostOnly(t *testing.T) {
	s := newAdminTestServer(t)
	s.SetConfig(ServerConfig{DataDir: t.TempDir()})
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/v1/backups/verify?path=x", nil)
		s.handleBackupsVerify(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for %s, got %d", m, w.Code)
		}
	}
}

// TestHandleRetentionReport_GetOnly 验证非 GET 返回 405。
func TestHandleRetentionReport_GetOnly(t *testing.T) {
	s := newAdminTestServer(t)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/v1/retention/acme/report", nil)
		s.handleRetentionReport(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for %s, got %d", m, w.Code)
		}
	}
}
