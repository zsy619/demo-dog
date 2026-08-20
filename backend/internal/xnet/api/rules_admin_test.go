package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
)

func setupRuleAdmin(t *testing.T) *Server {
	t.Helper()
	d := store.New(store.DefaultConfig())
	s := New(d, nil, nil)
	s.SetAuthMode(AuthModeOff)
	// Pre-seed one rule.
	s.alerts.eng.UpsertRule(alerts.Rule{
		Name:       "latency",
		Target:     0.99,
		Window:     time.Minute,
		FastWindow: 5 * time.Minute,
		FastBurn:   14.4,
		SlowBurn:   6,
		Severity:   alerts.SeverityWarning,
	})
	return s
}

func TestRulesAdmin_Get(t *testing.T) {
	s := setupRuleAdmin(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules/latency", nil)
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var r alerts.Rule
	if err := json.Unmarshal(rr.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Name != "latency" {
		t.Fatalf("name: %s", r.Name)
	}
}

func TestRulesAdmin_Get_Missing(t *testing.T) {
	s := setupRuleAdmin(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules/nope", nil)
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestRulesAdmin_Put_Create(t *testing.T) {
	s := setupRuleAdmin(t)
	body := mustJSON(t, alerts.Rule{
		Name: "errors", Target: 0.95, Window: time.Minute, Severity: alerts.SeverityCritical,
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/", bytes.NewReader(body))
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRulesAdmin_Put_Replace(t *testing.T) {
	s := setupRuleAdmin(t)
	body := mustJSON(t, alerts.Rule{
		Name: "latency", Target: 0.95, Window: 2 * time.Minute, Severity: alerts.SeverityCritical,
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/", bytes.NewReader(body))
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRulesAdmin_Put_EmptyName(t *testing.T) {
	s := setupRuleAdmin(t)
	body := mustJSON(t, alerts.Rule{Window: time.Minute})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/", bytes.NewReader(body))
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestRulesAdmin_Put_BadJSON(t *testing.T) {
	s := setupRuleAdmin(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/", bytes.NewReader([]byte("not json")))
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestRulesAdmin_Delete(t *testing.T) {
	s := setupRuleAdmin(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/rules/latency", nil)
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	if _, ok := s.alerts.eng.GetRule("latency"); ok {
		t.Fatal("rule should be gone")
	}
}

func TestRulesAdmin_Delete_Missing(t *testing.T) {
	s := setupRuleAdmin(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/rules/nope", nil)
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestRulesAdmin_MethodNotAllowed(t *testing.T) {
	s := setupRuleAdmin(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/rules/latency", nil)
	s.handleRulesAdmin(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", rr.Code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
