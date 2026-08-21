package tenants

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

// newTempKV 给测试提供一个临时目录 + KV 实例。
func newTempKV(t *testing.T) (xpersistence.KV, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = kv.Close() })
	return kv, path
}

// TestRegistry_Persistence_SurvivesRestart 验证进程重启后所有
// tenant + key 都被还原。
func TestRegistry_Persistence_SurvivesRestart(t *testing.T) {
	kv, path := newTempKV(t)
	ctx := context.Background()
	// 第一次进程:创建两个 tenant,各 mint 一个 key
	r1, err := NewWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	if _, err := r1.CreateTenant("acme", "Acme", "first"); err != nil {
		t.Fatalf("create acme: %v", err)
	}
	if _, err := r1.CreateTenant("zen", "Zen", "second"); err != nil {
		t.Fatalf("create zen: %v", err)
	}
	k1, err := r1.MintKey("acme", "checkout", "writer")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	k2, err := r1.MintKey("zen", "auth", "admin")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	// 模拟重启 —— 关掉 kv,重新打开同一份文件
	if err := kv.Close(); err != nil {
		t.Fatalf("close1: %v", err)
	}
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	// 验证 tenant
	if got, err := r2.Get("acme"); err != nil || got.Name != "Acme" {
		t.Errorf("reload acme: %v %v", got, err)
	}
	if got, err := r2.Get("zen"); err != nil || got.Name != "Zen" {
		t.Errorf("reload zen: %v %v", got, err)
	}
	// 验证 key 反向索引
	if r2.LookupTenant(k1.Plaintext) != "acme" {
		t.Errorf("reload k1: %s", r2.LookupTenant(k1.Plaintext))
	}
	if r2.LookupTenant(k2.Plaintext) != "zen" {
		t.Errorf("reload k2: %s", r2.LookupTenant(k2.Plaintext))
	}
	if len(r2.List()) != 2 {
		t.Errorf("list: got %d want 2", len(r2.List()))
	}
}

// TestRegistry_Persistence_UpdateAndDelete 验证 Update / Delete
// 都正确落盘并在重启后生效。
func TestRegistry_Persistence_UpdateAndDelete(t *testing.T) {
	kv, path := newTempKV(t)
	ctx := context.Background()
	r, err := NewWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t1, _ := r.CreateTenant("acme", "Acme", "before")
	// Update 名字
	t1.Name = "Acme Inc"
	if err := r.UpdateTenant(t1); err != nil {
		t.Fatalf("update: %v", err)
	}
	// 模拟重启
	if err := kv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen1: %v", err)
	}
	r2, err := NewWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	got, err := r2.Get("acme")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Acme Inc" {
		t.Errorf("update not persisted: %s", got.Name)
	}
	// Delete
	if err := r2.DeleteTenant("acme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := kv2.Close(); err != nil {
		t.Fatalf("close2: %v", err)
	}
	kv3, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen2: %v", err)
	}
	defer kv3.Close()
	r3, err := NewWithKV(ctx, kv3)
	if err != nil {
		t.Fatalf("new3: %v", err)
	}
	if _, err := r3.Get("acme"); err == nil {
		t.Errorf("tenant still exists after delete+restart")
	}
}

// TestRegistry_Persistence_NilKV 验证 nil KV 时退化为纯内存
// 模式,不 panic,也不写盘。
func TestRegistry_Persistence_NilKV(t *testing.T) {
	r := New()
	if _, err := r.CreateTenant("acme", "Acme", "x"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := r.MintKey("acme", "k", "writer"); err != nil {
		t.Fatalf("mint: %v", err)
	}
	// 没有 kv 也能 Get / List
	if got, err := r.Get("acme"); err != nil || got.Name != "Acme" {
		t.Errorf("get: %v %v", got, err)
	}
}

// TestRegistry_Persistence_DeleteCascadesKeys 验证 DeleteTenant
// 会同时清理该 tenant 下所有 key 反向索引。
func TestRegistry_Persistence_DeleteCascadesKeys(t *testing.T) {
	kv, path := newTempKV(t)
	ctx := context.Background()
	r, err := NewWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := r.CreateTenant("acme", "A", ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := r.CreateTenant("zen", "Z", ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	k1, _ := r.MintKey("acme", "k1", "writer")
	k2, _ := r.MintKey("zen", "k2", "writer")
	if err := r.DeleteTenant("acme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// k1 索引必须消失,k2 必须还在
	if r.LookupTenant(k1.Plaintext) != "" {
		t.Errorf("dangling key index for acme")
	}
	if r.LookupTenant(k2.Plaintext) != "zen" {
		t.Errorf("zen key disappeared")
	}
	// 重启后状态一致
	if err := kv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if r2.LookupTenant(k1.Plaintext) != "" {
		t.Errorf("after restart, dangling key index")
	}
	if r2.LookupTenant(k2.Plaintext) != "zen" {
		t.Errorf("after restart, zen key lost")
	}
}
