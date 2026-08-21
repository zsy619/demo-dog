package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

func newOIDCTestKV(t *testing.T) (xpersistence.KV, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "oidc.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return kv, path
}

// TestOIDCRegistry_Persistence_SurvivesRestart 验证 Upsert 后
// 的 OIDC provider 在进程重启后仍然能被 List 出来。
func TestOIDCRegistry_Persistence_SurvivesRestart(t *testing.T) {
	kv, path := newOIDCTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r1, err := NewOIDCRegistryWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	r1.Upsert(OIDCBundle{
		Issuer:      "https://dex.example.com",
		ClientID:    "demo-dog",
		Audiences:   []string{"demo-dog"},
		Scopes:      []string{"openid", "profile"},
		Enabled:     true,
		EmailClaim:  "email",
		GroupsClaim: "groups",
	})
	// 模拟重启
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewOIDCRegistryWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if got := r2.List(); len(got) != 1 {
		t.Fatalf("got %d providers, want 1", len(got))
	}
	if got := r2.List()[0]; got.Issuer != "https://dex.example.com" || got.ClientID != "demo-dog" {
		t.Errorf("wrong bundle: %+v", got)
	}
}

// TestOIDCRegistry_Persistence_DeleteClears 验证 Delete 后
// KV 中也清掉,重启后 List 不再返回该 issuer。
func TestOIDCRegistry_Persistence_DeleteClears(t *testing.T) {
	kv, path := newOIDCTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r, err := NewOIDCRegistryWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	r.Upsert(OIDCBundle{Issuer: "https://dex1.example.com", ClientID: "app1", Enabled: true})
	r.Upsert(OIDCBundle{Issuer: "https://dex2.example.com", ClientID: "app2", Enabled: true})
	if len(r.List()) != 2 {
		t.Fatalf("setup: want 2 got %d", len(r.List()))
	}
	r.Delete("https://dex1.example.com")
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewOIDCRegistryWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if got := r2.List(); len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if got := r2.List()[0]; got.Issuer != "https://dex2.example.com" {
		t.Errorf("wrong issuer: %s", got.Issuer)
	}
}

// TestOIDCRegistry_Persistence_UpsertOverwrites 验证同一 issuer
// 第二次 Upsert 覆盖旧值;重启后看到的是新值。
func TestOIDCRegistry_Persistence_UpsertOverwrites(t *testing.T) {
	kv, path := newOIDCTestKV(t)
	defer kv.Close()
	ctx := context.Background()
	r1, err := NewOIDCRegistryWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	r1.Upsert(OIDCBundle{Issuer: "https://dex.example.com", ClientID: "v1", Enabled: true})
	r1.Upsert(OIDCBundle{Issuer: "https://dex.example.com", ClientID: "v2", Scopes: []string{"openid"}})
	_ = kv.Close()
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	r2, err := NewOIDCRegistryWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if got := r2.List(); len(got) != 1 {
		t.Fatalf("got %d, want 1 (upsert should overwrite)", len(got))
	}
	if got := r2.List()[0]; got.ClientID != "v2" || len(got.Scopes) != 1 {
		t.Errorf("upsert did not overwrite: %+v", got)
	}
}

// TestOIDCRegistry_Persistence_NilKV 验证 nil KV 退化为纯内存。
func TestOIDCRegistry_Persistence_NilKV(t *testing.T) {
	r := NewOIDCRegistry()
	r.Upsert(OIDCBundle{Issuer: "https://dex.example.com", ClientID: "app", Enabled: true})
	if got := r.List(); len(got) != 1 {
		t.Errorf("nil KV should still keep in-memory: got %d", len(got))
	}
	r.Delete("https://dex.example.com")
	if got := r.List(); len(got) != 0 {
		t.Errorf("delete with nil KV should work: got %d", len(got))
	}
}
