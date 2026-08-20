package api

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestQuota_NoQuota_AlwaysAllowed(t *testing.T) {
	q := NewQuotaTracker()
	ok, _ := q.Allow("tenant-a", 1024)
	if !ok {
		t.Fatal("expected allow without quota")
	}
}

func TestQuota_RequestsAllowed(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxRequests: 3})
	for i := 0; i < 3; i++ {
		ok, _ := q.Allow("t", 0)
		if !ok {
			t.Fatalf("req %d: expected allow", i)
		}
	}
	ok, usage := q.Allow("t", 0)
	if ok {
		t.Fatal("expected reject on 4th")
	}
	if !usage.Limited {
		t.Fatal("usage should mark limited")
	}
}

func TestQuota_BytesAllowed(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxBytes: 100})
	if ok, _ := q.Allow("t", 80); !ok {
		t.Fatal("80 bytes should pass")
	}
	if ok, _ := q.Allow("t", 30); ok {
		t.Fatal("80+30=110 should fail")
	}
}

func TestQuota_LimitedRejectsUntilReset(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxRequests: 1})
	q.Allow("t", 0)
	ok, _ := q.Allow("t", 0)
	if ok {
		t.Fatal("expected first reject")
	}
	ok, _ = q.Allow("t", 0)
	if ok {
		t.Fatal("expected continued reject")
	}
}

func TestQuota_Reset(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxRequests: 1})
	q.Allow("t", 0)
	q.Reset("t")
	if ok, _ := q.Allow("t", 0); !ok {
		t.Fatal("after reset should allow")
	}
}

func TestQuota_Remove(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxRequests: 1})
	q.Allow("t", 0)
	q.Remove("t")
	if ok, _ := q.Allow("t", 0); !ok {
		t.Fatal("after remove should allow")
	}
}

func TestQuota_WindowRollover(t *testing.T) {
	q := NewQuotaTracker()
	fakeNow := time.Unix(1700000000, 0)
	q.now = func() time.Time { return fakeNow }
	q.Set(Quota{TenantID: "t", MaxRequests: 1, Window: time.Hour})
	q.Allow("t", 0)
	// Advance past the window.
	fakeNow = fakeNow.Add(time.Hour + time.Second)
	if ok, _ := q.Allow("t", 0); !ok {
		t.Fatal("new window should allow")
	}
}

func TestQuota_DefaultWindow(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxRequests: 1})
	q.Set(Quota{TenantID: "u"}) // no MaxRequests -> 0 = unlimited
	if ok, _ := q.Allow("u", 1<<30); !ok {
		t.Fatal("MaxRequests=0 means unlimited")
	}
}

func TestQuota_UsageSnapshot(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxRequests: 5})
	q.Allow("t", 100)
	q.Allow("t", 200)
	u, ok := q.Usage("t")
	if !ok {
		t.Fatal("usage should exist")
	}
	if u.Requests != 2 {
		t.Fatalf("requests: %d", u.Requests)
	}
	if u.Bytes != 300 {
		t.Fatalf("bytes: %d", u.Bytes)
	}
	if u.MaxRequests != 5 {
		t.Fatalf("max: %d", u.MaxRequests)
	}
}

func TestQuota_Usage_Missing(t *testing.T) {
	q := NewQuotaTracker()
	_, ok := q.Usage("never-seen")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestQuota_Snapshot(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "a", MaxRequests: 10})
	q.Set(Quota{TenantID: "b", MaxRequests: 20})
	q.Allow("a", 5)
	q.Allow("b", 10)
	snap := q.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2, got %d", len(snap))
	}
}

func TestQuota_Prometheus_EmitsAll(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 100, MaxBytes: 1024})
	q.Allow("acme", 100)
	q.Allow("acme", 200)
	var buf bytes.Buffer
	q.WritePrometheus(&buf)
	out := buf.String()
	for _, want := range []string{
		"dog_tenant_quota_requests",
		"dog_tenant_quota_bytes",
		"dog_tenant_quota_limited",
		"dog_tenant_quota_max_requests",
		"dog_tenant_quota_max_bytes",
		`tenant="acme"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output", want)
		}
	}
}

func TestQuota_Prometheus_LimitedFlag(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "t", MaxRequests: 1})
	q.Allow("t", 0)
	q.Allow("t", 0) // exhausts
	var buf bytes.Buffer
	q.WritePrometheus(&buf)
	if !strings.Contains(buf.String(), `dog_tenant_quota_limited{tenant="t"} 1`) {
		t.Fatalf("expected limited=1, got: %s", buf.String())
	}
}

func TestQuota_Prometheus_EmptyTracker(t *testing.T) {
	q := NewQuotaTracker()
	var buf bytes.Buffer
	q.WritePrometheus(&buf)
	if !strings.Contains(buf.String(), "# HELP") {
		t.Fatal("expected HELP even with no tenants")
	}
}
