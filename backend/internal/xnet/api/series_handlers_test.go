package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/ingest"
	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
	"github.com/zsy619/demo-dog/backend/internal/xflow/stream"
)

func newSeriesTestServer(t *testing.T) *Server {
	t.Helper()
	d := store.New(store.Config{
		HotLogTTL:    5 * time.Minute,
		HotLogCap:    2048,
		HotMetricCap: 4096,
		ColdCap:      10_000,
	})
	hub := stream.NewHub()
	in := ingest.New(d, 4)
	s := New(d, in, hub)
	s.SetAuthMode(AuthModeOff)
	now := time.Now()
	in.SubmitSync(model.OTLPRequest{
		TenantID:      "acme",
		ResourceAttrs: map[string]string{"service.name": "checkout"},
		Metrics: []model.MetricPoint{
			{TenantID: "acme", Service: "checkout", Name: "http.server.duration", Timestamp: now, Value: 100, Labels: map[string]string{"route": "/pay", "code": "200"}},
			{TenantID: "acme", Service: "checkout", Name: "http.server.duration", Timestamp: now, Value: 110, Labels: map[string]string{"route": "/pay", "code": "200"}},
			{TenantID: "acme", Service: "checkout", Name: "http.server.duration", Timestamp: now, Value: 220, Labels: map[string]string{"route": "/refund", "code": "500"}},
			{TenantID: "acme", Service: "checkout", Name: "requests_total", Timestamp: now, Value: 1},
		},
	})
	return s
}

func TestHandleSeries(t *testing.T) {
	s := newSeriesTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/series", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Status string `json:"status"`
		Data   []struct {
			Name string `json:"__name__"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("status=%s", resp.Status)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 metric names, got %d: %v", len(resp.Data), resp.Data)
	}
}

func TestHandleSeries_MatchFilter(t *testing.T) {
	s := newSeriesTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/series?match[]=requests_total", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var resp struct {
		Data []struct {
			Name string `json:"__name__"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].Name != "requests_total" {
		t.Fatalf("expected 1 row, got %+v", resp.Data)
	}
}

func TestHandleMetadata(t *testing.T) {
	s := newSeriesTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/metadata", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Status string                            `json:"status"`
		Data   map[string][]struct {
			Type string `json:"type"`
			Help string `json:"help"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("status=%s", resp.Status)
	}
	if _, ok := resp.Data["requests_total"]; !ok {
		t.Fatal("requests_total missing")
	}
	if resp.Data["requests_total"][0].Type != "counter" {
		t.Fatalf("requests_total kind=%s", resp.Data["requests_total"][0].Type)
	}
}

func TestParseMatchName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http.server.duration", "http.server.duration"},
		{`{__name__="http.server.duration"}`, "http.server.duration"},
		{"{__name__=\"foo\",__name__=\"bar\"}", "foo"},
		{`{__name__=""}`, ""},
	}
	for _, c := range cases {
		got := parseMatchName(c.in)
		if got != c.want {
			t.Errorf("parseMatchName(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}
