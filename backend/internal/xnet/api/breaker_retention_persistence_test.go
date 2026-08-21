package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xcache/circuit"
	"github.com/zsy619/demo-dog/backend/internal/xdata/retention"
	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

func newSubsystemTestKV(t *testing.T) (xpersistence.KV, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "control.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return kv, path
}

// ---- BreakerRegistry ----

// TestBreakerRegistry_Persistence_GetPersists 验证 Get() 创建
// 的断路器立即落盘,重启后 All() 仍能列出。
func TestBreakerRegistry_Persistence_GetPersists(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r1, err := NewBreakerRegistryWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	r1.Get("ingest")
	r1.Get("queries")
	// 模拟重启
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewBreakerRegistryWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	all := r2.All()
	if len(all) != 2 {
		t.Fatalf("got %d, want 2", len(all))
	}
	if _, ok := all["ingest"]; !ok {
		t.Errorf("ingest missing after restart")
	}
	if _, ok := all["queries"]; !ok {
		t.Errorf("queries missing after restart")
	}
}

// TestBreakerRegistry_Persistence_FailurePersists 验证一个
// 失败计数累积后,重启时通过 Restore 还原。
func TestBreakerRegistry_Persistence_FailurePersists(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r1, err := NewBreakerRegistryWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	b := r1.Get("ingest")
	// 触发 3 次失败,Snapshot 应为 failures=3
	for i := 0; i < 3; i++ {
		b.Failure()
	}
	// 显式持久化(Get 之后再 persist 一次以确保)
	_ = r1.persist("ingest", b)
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewBreakerRegistryWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	snap := r2.All()["ingest"]
	if snap.Failures != 3 {
		t.Errorf("failures not restored: got %d want 3", snap.Failures)
	}
}

// TestBreakerRegistry_Persistence_ResetPersists 验证 Reset
// 之后状态也是持久化的。
func TestBreakerRegistry_Persistence_ResetPersists(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r1, err := NewBreakerRegistryWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	b := r1.Get("ingest")
	// 触发 5 次失败让 breaker 跳闸
	for i := 0; i < 5; i++ {
		b.Failure()
	}
	if b.State() != circuit.StateOpen {
		t.Fatalf("expected open, got %v", b.State())
	}
	// 重置
	r1.Reset("ingest")
	if b.State() != circuit.StateClosed {
		t.Fatalf("expected closed after reset, got %v", b.State())
	}
	// 模拟重启
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewBreakerRegistryWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	snap := r2.All()["ingest"]
	if snap.State != "closed" {
		t.Errorf("reset state not persisted: got %s", snap.State)
	}
}

// TestBreakerRegistry_Persistence_NilKV 验证 nil KV 退化为
// 纯内存模式,不影响现有调用方。
func TestBreakerRegistry_Persistence_NilKV(t *testing.T) {
	r := NewBreakerRegistry()
	b := r.Get("test")
	b.Failure()
	all := r.All()
	if all["test"].Failures != 1 {
		t.Errorf("nil KV should still work in-memory: %+v", all["test"])
	}
	r.Reset("test")
	if all := r.All(); all["test"].Failures != 0 {
		t.Errorf("reset should work in-memory too: %+v", all["test"])
	}
}

// ---- RetentionManager ----

// TestRetention_Persistence_SetPolicyPersists 验证 SetPolicy
// 后进程重启仍可读到。
func TestRetention_Persistence_SetPolicyPersists(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r1, err := NewRetentionManagerWithKV(ctx, kv, "")
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	if err := r1.SetPolicy(retention.Policy{
		Tenant:  "acme",
		Tier:    retention.TierPro,
		HotTTL:  3 * 24 * time.Hour,
		ColdTTL: 30 * 24 * time.Hour,
	}); err != nil {
		t.Fatalf("setpolicy: %v", err)
	}
	if err := r1.SetPolicy(retention.Policy{
		Tenant:  "zen",
		Tier:    retention.TierFree,
		HotTTL:  24 * time.Hour,
		ColdTTL: 7 * 24 * time.Hour,
	}); err != nil {
		t.Fatalf("setpolicy2: %v", err)
	}
	// 模拟重启
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewRetentionManagerWithKV(ctx, kv2, "")
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if got, ok := r2.Manager().Get("acme"); !ok || got.Tier != retention.TierPro {
		t.Errorf("acme missing or wrong tier: %+v", got)
	}
	if got, ok := r2.Manager().Get("zen"); !ok || got.Tier != retention.TierFree {
		t.Errorf("zen missing or wrong tier: %+v", got)
	}
}

// TestRetention_Persistence_RemoveClears 验证 Remove 后 KV
// 中也清掉。
func TestRetention_Persistence_RemoveClears(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r1, err := NewRetentionManagerWithKV(ctx, kv, "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := r1.SetPolicy(retention.Policy{
		Tenant:  "acme",
		Tier:    retention.TierPro,
		HotTTL:  time.Hour,
		ColdTTL: 24 * time.Hour,
	}); err != nil {
		t.Fatal(err)
	}
	r1.Remove("acme")
	// 模拟重启
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewRetentionManagerWithKV(ctx, kv2, "")
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if _, ok := r2.Manager().Get("acme"); ok {
		t.Errorf("removed policy still present after restart")
	}
}

// TestRetention_Persistence_NilKV 验证 nil KV 退化为纯内存。
func TestRetention_Persistence_NilKV(t *testing.T) {
	r := NewRetentionManager()
	if err := r.SetPolicy(retention.Policy{
		Tenant:  "acme",
		Tier:    retention.TierPro,
		HotTTL:  time.Hour,
		ColdTTL: 24 * time.Hour,
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Manager().Get("acme"); !ok {
		t.Errorf("nil KV should still work")
	}
	r.Remove("acme")
	if _, ok := r.Manager().Get("acme"); ok {
		t.Errorf("remove with nil KV should work")
	}
}
