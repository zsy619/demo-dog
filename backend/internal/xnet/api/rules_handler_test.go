package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
	"github.com/zsy619/demo-dog/backend/internal/xdata/ingest"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
	"github.com/zsy619/demo-dog/backend/internal/xflow/stream"
)

func TestHandleRules_Empty(t *testing.T) {
	d := store.New(store.DefaultConfig())
	hub := stream.NewHub()
	s := New(d, ingest.New(d, 2), hub)
	s.SetAuthMode(AuthModeOff)
	req := httptest.NewRequest("GET", "/api/v1/rules", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Groups []json.RawMessage `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("status=%s", resp.Status)
	}
	if len(resp.Data.Groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(resp.Data.Groups))
	}
}

func TestHandleRules_WithRules(t *testing.T) {
	d := store.New(store.DefaultConfig())
	hub := stream.NewHub()
	s := New(d, ingest.New(d, 2), hub)
	s.SetAuthMode(AuthModeOff)
	s.alerts.eng.SetRules([]alerts.Rule{{
		Name:        "checkout-availability",
		Description: "99.9% availability for checkout",
		Service:     "checkout",
		Target:      0.999,
		Window:      time.Hour,
		FastWindow:  5 * time.Minute,
		FastBurn:    14.4,
		SlowBurn:    1,
		Severity:    alerts.SeverityCritical,
		Channels:    []string{"pagerduty"},
	}})
	req := httptest.NewRequest("GET", "/api/v1/rules", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Groups []struct {
				Name string `json:"name"`
				File string `json:"file"`
				Rules []struct {
					Name string `json:"name"`
					Type string `json:"type"`
					Health string `json:"health"`
				} `json:"rules"`
			} `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(resp.Data.Groups))
	}
	g := resp.Data.Groups[0]
	if g.Name != "checkout-availability" {
		t.Fatalf("name=%s", g.Name)
	}
	if g.File != "slo.burnrate" {
		t.Fatalf("file=%s", g.File)
	}
	if len(g.Rules) != 1 || g.Rules[0].Name != "checkout-availability" {
		t.Fatalf("rule missing")
	}
	if g.Rules[0].Type != "alerting" {
		t.Fatalf("type=%s", g.Rules[0].Type)
	}
	if g.Rules[0].Health != "ok" {
		t.Fatalf("health=%s", g.Rules[0].Health)
	}
}
