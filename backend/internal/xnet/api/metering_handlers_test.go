package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xbilling"
)

func newMeteringTestServer(t *testing.T) (*Server, xbilling.Meter) {
	t.Helper()
	s := newAdminTestServer(t)
	counter := xbilling.NewCounter()
	s.SetMeter(counter)
	return s, counter
}

func doMeterReq(t *testing.T, s *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	rw := httptest.NewRecorder()
	// CSV 端点只看 path 前缀是否含 usage.csv
	if strings.Contains(target, "usage.csv") {
		s.handleUsageCSV(rw, r)
		return rw
	}
	s.handleUsage(rw, r)
	return rw
}

func TestHandleUsage_PostThenGet(t *testing.T) {
	s, counter := newMeteringTestServer(t)

	body := `{"tenant":"acme","metric":"invocations","delta":42,"at":"2026-03-05T10:00:00Z"}`
	rr := doMeterReq(t, s, http.MethodPost, "/api/v1/billing/usage", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST status: %d body=%s", rr.Code, rr.Body.String())
	}
	if v, _ := counter.Query("acme", "invocations", "2026-03"); v != 42 {
		t.Errorf("POST not recorded in counter: got %d", v)
	}

	get := doMeterReq(t, s, http.MethodGet, "/api/v1/billing/usage?tenant=acme&period=2026-03&metric=invocations", "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET status: %d", get.Code)
	}
	var got map[string]any
	if err := json.NewDecoder(get.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["value"].(float64) != 42 {
		t.Errorf("value: %v", got["value"])
	}
	if got["present"].(bool) != true {
		t.Errorf("present: %v", got["present"])
	}
}

func TestHandleUsage_QueryTenant(t *testing.T) {
	s, counter := newMeteringTestServer(t)
	counter.Record("acme", "invocations", 5, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	counter.Record("acme", "invocations", 7, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	counter.Record("acme", "bytes_in", 100, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	rr := doMeterReq(t, s, http.MethodGet, "/api/v1/billing/usage?tenant=acme", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var got struct {
		Tenant string         `json:"tenant"`
		Usage  []xbilling.Usage `json:"usage"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Tenant != "acme" {
		t.Errorf("tenant: %s", got.Tenant)
	}
	if len(got.Usage) != 2 {
		t.Fatalf("want 2 metrics, got %d: %+v", len(got.Usage), got.Usage)
	}
}

func TestHandleUsageCSV(t *testing.T) {
	s, counter := newMeteringTestServer(t)
	at := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	counter.Record("acme", "invocations", 100, at)
	counter.Record("zen", "bytes_in", 1024, at)

	rr := doMeterReq(t, s, http.MethodGet, "/api/v1/billing/usage.csv", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content-type: %s", ct)
	}
	body := rr.Body.String()
	if !strings.HasPrefix(body, "period,tenant,metric,value,updated_at\n") {
		t.Errorf("missing csv header: %q", body[:min(60, len(body))])
	}
	if !strings.Contains(body, "2026-03,acme,invocations,100,") {
		t.Errorf("missing acme row: %s", body)
	}
	if !strings.Contains(body, "2026-03,zen,bytes_in,1024,") {
		t.Errorf("missing zen row: %s", body)
	}
}

func TestHandleUsageCSV_FilterByTenant(t *testing.T) {
	s, counter := newMeteringTestServer(t)
	at := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	counter.Record("acme", "invocations", 100, at)
	counter.Record("zen", "invocations", 200, at)

	rr := doMeterReq(t, s, http.MethodGet, "/api/v1/billing/usage.csv?tenant=zen", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, ",acme,") {
		t.Errorf("tenant filter should exclude acme: %s", body)
	}
	if !strings.Contains(body, "2026-03,zen,invocations,200,") {
		t.Errorf("missing zen row: %s", body)
	}
}

func TestHandleUsage_NoMeter(t *testing.T) {
	s := newAdminTestServer(t)
	rr := doMeterReq(t, s, http.MethodGet, "/api/v1/billing/usage?tenant=acme", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "recorded") {
		t.Errorf("expected recorded marker, got: %s", body)
	}
}

func TestHandleUsage_BadInputs(t *testing.T) {
	s, _ := newMeteringTestServer(t)
	// empty tenant
	rr := doMeterReq(t, s, http.MethodPost, "/api/v1/billing/usage",
		`{"tenant":"","metric":"x","delta":1}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("empty tenant: %d", rr.Code)
	}
	// delta=0
	rr = doMeterReq(t, s, http.MethodPost, "/api/v1/billing/usage",
		`{"tenant":"acme","metric":"x","delta":0}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("delta=0: %d", rr.Code)
	}
	// bad json
	rr = doMeterReq(t, s, http.MethodPost, "/api/v1/billing/usage",
		`{not-json`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad json: %d", rr.Code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
