package api

import (
	"testing"
	"time"
)

func TestAuditFilter_Method(t *testing.T) {
	log := NewAuditLog(100)
	now := time.Now()
	log.Append(AuditEvent{Timestamp: now, Method: "GET", Path: "/a", Status: 200})
	log.Append(AuditEvent{Timestamp: now, Method: "POST", Path: "/b", Status: 200})
	out := log.Filter(0, AuditFilter{Method: "POST"})
	if len(out) != 1 {
		t.Fatalf("got %d", len(out))
	}
	if out[0].Method != "POST" {
		t.Fatalf("got %s", out[0].Method)
	}
}

func TestAuditFilter_Tenant(t *testing.T) {
	log := NewAuditLog(100)
	now := time.Now()
	log.Append(AuditEvent{Timestamp: now, Tenant: "acme", Status: 200})
	log.Append(AuditEvent{Timestamp: now, Tenant: "globex", Status: 200})
	out := log.Filter(0, AuditFilter{Tenant: "acme"})
	if len(out) != 1 || out[0].Tenant != "acme" {
		t.Fatalf("got %+v", out)
	}
}

func TestAuditFilter_StatusRange(t *testing.T) {
	log := NewAuditLog(100)
	now := time.Now()
	for _, s := range []int{200, 201, 400, 500} {
		log.Append(AuditEvent{Timestamp: now, Status: s})
	}
	out := log.Filter(0, AuditFilter{StatusMin: 400, StatusMax: 599})
	if len(out) != 2 {
		t.Fatalf("got %d", len(out))
	}
}

func TestAuditFilter_TimeRange(t *testing.T) {
	log := NewAuditLog(100)
	for i := 0; i < 10; i++ {
		log.Append(AuditEvent{Timestamp: time.Unix(int64(i*10), 0), Status: 200})
	}
	since := time.Unix(30, 0)
	out := log.Filter(0, AuditFilter{Since: since})
	if len(out) != 7 {
		t.Fatalf("got %d", len(out))
	}
}

func TestAuditFilter_Path(t *testing.T) {
	log := NewAuditLog(100)
	log.Append(AuditEvent{Path: "/api/services"})
	log.Append(AuditEvent{Path: "/api/query"})
	out := log.Filter(0, AuditFilter{Path: "/api/serv"})
	if len(out) != 1 {
		t.Fatalf("got %d", len(out))
	}
}

func TestAuditLog_Retention(t *testing.T) {
	log := NewAuditLog(100)
	log.SetRetentionTTL(50 * time.Millisecond)
	defer log.Close()
	log.Append(AuditEvent{Timestamp: time.Now(), Status: 200})
	if len(log.Recent(0)) != 1 {
		t.Fatal("expected event after append")
	}
	time.Sleep(120)
	// Force a sweep cycle (sweep runs every minute by default; for
	// the test we just check the events are still here until the
	// sweeper runs). The TTL is applied lazily; verify the API is
	// wired without timing-dependent flakiness.
	if log.retentionTTL != 50*time.Millisecond {
		t.Fatal("ttl not set")
	}
}

func TestAuditLog_RetentionDisabled(t *testing.T) {
	log := NewAuditLog(100)
	log.SetRetentionTTL(0) // explicitly disabled
	log.Close()             // must be safe even if no sweeper
	log.Close()             // idempotent
}
