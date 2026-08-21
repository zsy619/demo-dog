package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

func newAdminTestKV(t *testing.T) (xpersistence.KV, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "admin.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return kv, path
}

// TestAdminStore_Persistence_SurvivesRestart 验证 CreateKey
// 后的 entry 在进程重启后仍然能被 LookupByToken 找到。
func TestAdminStore_Persistence_SurvivesRestart(t *testing.T) {
	kv, path := newAdminTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	// 第一次进程
	s1, err := NewAdminStoreWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	raw, entry, err := s1.CreateKey("reader", "checkout-svc", "acme", []string{"logs:read"}, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if entry.KeyID == "" {
		t.Fatalf("entry.KeyID empty")
	}
	if err := kv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// 重启
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	s2, err := NewAdminStoreWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	got, ok := s2.LookupByToken(raw)
	if !ok {
		t.Fatalf("token not found after restart")
	}
	if got.KeyID != entry.KeyID {
		t.Errorf("KeyID mismatch: got %s want %s", got.KeyID, entry.KeyID)
	}
	if got.Label != "checkout-svc" {
		t.Errorf("label mismatch: %s", got.Label)
	}
}

// TestAdminStore_Persistence_DisablePersists 验证 DisableKey
// 后重启,Disabled 字段仍然保留。
func TestAdminStore_Persistence_DisablePersists(t *testing.T) {
	kv, path := newAdminTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	s, err := NewAdminStoreWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	raw, entry, err := s.CreateKey("admin", "ops", "acme", nil, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.DisableKey(entry.KeyID); err != nil {
		t.Fatalf("disable: %v", err)
	}
	// 重启
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	s2, err := NewAdminStoreWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	got, ok := s2.LookupByToken(raw)
	if !ok {
		t.Fatalf("not found after restart")
	}
	if !got.Disabled {
		t.Errorf("Disabled not persisted")
	}
}

// TestAdminStore_Persistence_DeleteClears 验证 DeleteKey 后
// KV 中也清掉,重启后 LookupByToken 返回 false。
func TestAdminStore_Persistence_DeleteClears(t *testing.T) {
	kv, path := newAdminTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	s, err := NewAdminStoreWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	raw, entry, _ := s.CreateKey("writer", "", "acme", nil, 0)
	if err := s.DeleteKey(entry.KeyID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	s2, _ := NewAdminStoreWithKV(ctx, kv2)
	if _, ok := s2.LookupByToken(raw); ok {
		t.Errorf("deleted key still resolves after restart")
	}
}

// TestAdminStore_Persistence_RotatePersists 验证 RotateKey
// 后的新 key 落盘,重启后能查到;旧 key 也保留 ExpiresAt。
func TestAdminStore_Persistence_RotatePersists(t *testing.T) {
	kv, path := newAdminTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	s, _ := NewAdminStoreWithKV(ctx, kv)
	raw1, entry1, _ := s.CreateKey("writer", "", "acme", nil, 0)
	raw2, _, newEntry, err := s.RotateKey(entry1.KeyID, time.Hour)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if raw1 == raw2 {
		t.Fatalf("rotate produced same raw token")
	}
	// 重启
	_ = kv.Close()
	kv2, _ := xpersistence.OpenFileJSON(path)
	defer kv2.Close()
	s2, _ := NewAdminStoreWithKV(ctx, kv2)
	// 新 key 仍然解析
	if _, ok := s2.LookupByToken(raw2); !ok {
		t.Errorf("new key not found after restart")
	}
	// 旧 key 仍然存在,但 ExpiresAt 已设置
	oldGot, ok := s2.LookupByToken(raw1)
	if !ok {
		t.Errorf("old key not found after restart")
	} else if oldGot.ExpiresAt.IsZero() {
		t.Errorf("old key ExpiresAt not persisted")
	}
	// RotatedFrom 关系保留
	gotNew, _ := s2.LookupByID(newEntry.KeyID)
	if gotNew == nil || gotNew.RotatedFrom != entry1.KeyID {
		t.Errorf("RotatedFrom not persisted: %+v", gotNew)
	}
}
