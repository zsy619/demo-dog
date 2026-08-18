package tenants

import (
	"strings"
	"testing"
)

func TestRegistry_CreateAndGet(t *testing.T) {
	r := New()
	t1, err := r.CreateTenant("acme", "Acme", "demo corp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if t1.ID != "acme" {
		t.Errorf("expected id acme, got %q", t1.ID)
	}
	got, err := r.Get("acme")
	if err != nil || got.Name != "Acme" {
		t.Errorf("get: %v %v", got, err)
	}
	if _, err := r.Get("missing"); err == nil {
		t.Errorf("expected error for missing tenant")
	}
}

func TestRegistry_Duplicate(t *testing.T) {
	r := New()
	if _, err := r.CreateTenant("acme", "A", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateTenant("acme", "A2", "x"); err == nil {
		t.Errorf("expected duplicate error")
	}
}

func TestRegistry_MintAndLookup(t *testing.T) {
	r := New()
	if _, err := r.CreateTenant("acme", "A", "x"); err != nil {
		t.Fatal(err)
	}
	k, err := r.MintKey("acme", "checkout", "writer")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(k.Plaintext, "dog_") {
		t.Errorf("key prefix missing")
	}
	if r.LookupTenant(k.Plaintext) != "acme" {
		t.Errorf("lookup failed")
	}
}

func TestRegistry_MintMissingTenant(t *testing.T) {
	r := New()
	if _, err := r.MintKey("ghost", "x", "writer"); err == nil {
		t.Errorf("expected error")
	}
}

func TestRegistry_Snapshot(t *testing.T) {
	r := New()
	if _, err := r.CreateTenant("acme", "A", "x"); err != nil {
		t.Fatal(err)
	}
	k, _ := r.MintKey("acme", "checkout", "writer")
	snap := r.SnapshotKeyMap()
	if snap[k.Plaintext] != "acme" {
		t.Errorf("snapshot missing key")
	}
}
