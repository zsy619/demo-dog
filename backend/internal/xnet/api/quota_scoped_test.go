package api

import (
	"testing"
	"time"
)

// W1.6: 配额按 (tenant, scope) 隔离。

func TestQuota_AllowScoped_IndependentBuckets(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 2})
	// ingest 桶用完 2 个请求
	if ok, _ := q.AllowScoped("acme", "ingest", 0); !ok {
		t.Fatal("ingest #1 must pass")
	}
	if ok, _ := q.AllowScoped("acme", "ingest", 0); !ok {
		t.Fatal("ingest #2 must pass")
	}
	if ok, _ := q.AllowScoped("acme", "ingest", 0); ok {
		t.Fatal("ingest #3 must reject")
	}
	// query 桶应独立,还能正常通过
	if ok, _ := q.AllowScoped("acme", "query", 0); !ok {
		t.Fatal("query scope must not be affected by ingest")
	}
	// billing 同理
	if ok, _ := q.AllowScoped("acme", "billing", 0); !ok {
		t.Fatal("billing scope must not be affected by ingest")
	}
}

func TestQuota_AllowScoped_FallbackToTenantDefault(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 1})
	// 显式 scope 与未配置,但 tenant 配额只有 1,故应立刻受限。
	if ok, _ := q.AllowScoped("acme", "", 0); !ok {
		t.Fatal("first empty-scope call must pass")
	}
	if ok, _ := q.AllowScoped("acme", "", 0); ok {
		t.Fatal("second empty-scope call must reject")
	}
}

func TestQuota_AllowScoped_EmptyTenantAllowed(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 1})
	if ok, _ := q.AllowScoped("", "anything", 0); !ok {
		t.Fatal("empty tenant must always be allowed")
	}
}

func TestQuota_AllowScoped_UsageSnapshot(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 100})
	q.AllowScoped("acme", "ingest", 0)
	q.AllowScoped("acme", "ingest", 0)
	q.AllowScoped("acme", "query", 0)
	if u, ok := q.UsageScoped("acme", "ingest"); !ok || u.Requests != 2 || u.Scope != "ingest" {
		t.Fatalf("ingest usage wrong: %+v ok=%v", u, ok)
	}
	if u, ok := q.UsageScoped("acme", "query"); !ok || u.Requests != 1 || u.Scope != "query" {
		t.Fatalf("query usage wrong: %+v ok=%v", u, ok)
	}
	if u, ok := q.UsageScoped("acme", "billing"); ok {
		t.Fatalf("unused scope must not exist: %+v", u)
	}
}

func TestQuota_AllowScoped_WindowRolloverPerScope(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 1, Window: time.Hour})
	q.AllowScoped("acme", "ingest", 0) // bucket created
	if ok, _ := q.AllowScoped("acme", "query", 0); !ok {
		t.Fatal("query scope must allow initially")
	}
}

func TestQuota_RemoveScopeBuckets(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 1})
	q.AllowScoped("acme", "ingest", 0)
	q.AllowScoped("acme", "ingest", 0) // reject
	if u, ok := q.UsageScoped("acme", "ingest"); !ok || !u.Limited {
		t.Fatalf("want limited, got %+v ok=%v", u, ok)
	}
	q.Reset("acme")
	// Reset 之后 bucket 必须消失(ok=false),
	// 下一个 AllowScoped 必须从零重新计起。
	if _, ok := q.UsageScoped("acme", "ingest"); ok {
		t.Fatalf("Reset must clear bucket")
	}
	if ok, _ := q.AllowScoped("acme", "ingest", 0); !ok {
		t.Fatal("post-reset call must allow (bucket freshly created)")
	}
}

func TestQuota_SetClearsAllScopeBuckets(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{TenantID: "acme", MaxRequests: 1})
	q.AllowScoped("acme", "ingest", 0)
	q.AllowScoped("acme", "query", 0)
	// 重新设配额必须清掉所有 scope 的 bucket
	q.Set(Quota{TenantID: "acme", MaxRequests: 100})
	if u, ok := q.UsageScoped("acme", "ingest"); ok {
		t.Fatalf("ingest bucket must be cleared by Set, got %+v", u)
	}
	if u, ok := q.UsageScoped("acme", "query"); ok {
		t.Fatalf("query bucket must be cleared by Set, got %+v", u)
	}
}
