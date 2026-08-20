package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAudit_AppendAndRecent(t *testing.T) {
	log := NewAuditLog(4)
	for i := 0; i < 7; i++ {
		log.Append(AuditEvent{Path: "/x", Status: 200})
	}
	recent := log.Recent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 events, got %d", len(recent))
	}
	// After wrap-around the most recent three events are stored at
	// positions 4, 5, 6 — i.e. the last three appended.
	if recent[0].Path != "/x" || recent[2].Path != "/x" {
		t.Errorf("unexpected ordering: %+v", recent)
	}
}

func TestAudit_RecentAll(t *testing.T) {
	log := NewAuditLog(0) // default 10k cap
	for i := 0; i < 5; i++ {
		log.Append(AuditEvent{Path: "/y"})
	}
	all := log.Recent(0)
	if len(all) != 5 {
		t.Fatalf("expected 5, got %d", len(all))
	}
}

func TestAudit_Stats(t *testing.T) {
	log := NewAuditLog(2)
	log.Append(AuditEvent{})
	log.Append(AuditEvent{})
	log.Append(AuditEvent{}) // overflow
	stats := log.Stats()
	if got := stats["capacity"]; got != 2 {
		t.Errorf("cap = %v, want 2", got)
	}
	if got := stats["buffered"]; got != 2 {
		t.Errorf("buffered = %v, want 2", got)
	}
	if got := stats["total"]; got != uint64(3) {
		t.Errorf("total = %v, want 3", got)
	}
}

func TestAudit_EncodeJSON(t *testing.T) {
	log := NewAuditLog(10)
	log.Append(AuditEvent{Method: "POST", Path: "/api/ingest/otlp", Status: 200})
	data, err := log.EncodeJSON()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty JSON")
	}
}

// Make sure the audit middleware actually wraps a handler.
func TestAudit_MiddlewareRecords(t *testing.T) {
	log := NewAuditLog(100)
	h := AuditMiddleware(log, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		w.Write([]byte("hi"))
	}))

	// POST should be recorded.
	req := httptest.NewRequest("POST", "/api/ingest/otlp", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 201 {
		t.Fatalf("handler got %d", rr.Code)
	}
	recents := log.Recent(10)
	if len(recents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(recents))
	}
	if recents[0].Method != "POST" {
		t.Errorf("method = %q", recents[0].Method)
	}
	if recents[0].Status != 201 {
		t.Errorf("status = %d", recents[0].Status)
	}

	// GET should NOT be recorded (recordReads=false).
	req = httptest.NewRequest("GET", "/api/services", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if len(log.Recent(10)) != 1 {
		t.Errorf("GET should not be recorded")
	}
}

// recordReads=true records GET too.
func TestAudit_MiddlewareRecordReads(t *testing.T) {
	log := NewAuditLog(100)
	h := AuditMiddleware(log, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/api/services", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if len(log.Recent(10)) != 1 {
		t.Fatal("recordReads=true should record GET")
	}
}
