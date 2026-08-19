package auth

import (
	"strings"
	"testing"
	"time"
)

func TestKeyEntry_IsValid(t *testing.T) {
	now := time.Now()
	e := &KeyEntry{CreatedAt: now}
	if !e.IsValid(now) {
		t.Fatal("fresh entry should be valid")
	}
	e.Disabled = true
	if e.IsValid(now) {
		t.Fatal("disabled should be invalid")
	}
	e.Disabled = false
	e.ExpiresAt = now.Add(-time.Hour)
	if e.IsValid(now) {
		t.Fatal("expired should be invalid")
	}
	e.ExpiresAt = now.Add(time.Hour)
	if !e.IsValid(now) {
		t.Fatal("future-expiry should be valid")
	}
}

func TestKeyEntry_HasScope(t *testing.T) {
	// Empty scopes = legacy, matches everything.
	e := &KeyEntry{}
	if !e.HasScope("rules:read") {
		t.Fatal("empty scopes should be permissive")
	}
	e.Scopes = []string{"rules:read", "metrics:write"}
	if !e.HasScope("rules:read") {
		t.Fatal("expected match")
	}
	if e.HasScope("logs:read") {
		t.Fatal("expected miss")
	}
}

func TestKeyEntry_HasResourceScope(t *testing.T) {
	e := &KeyEntry{Scopes: []string{"rules:read"}}
	if !e.HasResourceScope("rules") {
		t.Fatal("rules:read should grant rules resource access")
	}
	if e.HasResourceScope("logs") {
		t.Fatal("no logs scope")
	}
	e.Scopes = []string{"logs:write"}
	if !e.HasResourceScope("logs") {
		t.Fatal("logs:write should grant logs resource access")
	}
}

func TestAdminStore_CreateAndLookup(t *testing.T) {
	s := NewAdminStore()
	raw, entry, err := s.CreateKey("admin", "acme", []string{"rules:read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		t.Fatal("expected raw token")
	}
	if entry.KeyID == "" {
		t.Fatal("expected KeyID")
	}
	if entry.Hash == "" {
		t.Fatal("expected Hash")
	}
	got, ok := s.LookupByToken(raw)
	if !ok {
		t.Fatal("lookup failed")
	}
	if got.KeyID != entry.KeyID {
		t.Fatalf("KeyID mismatch")
	}
}

func TestAdminStore_LookupByToken_Wrong(t *testing.T) {
	s := NewAdminStore()
	if _, ok := s.LookupByToken("bogus"); ok {
		t.Fatal("expected miss")
	}
}

func TestAdminStore_LookupByID(t *testing.T) {
	s := NewAdminStore()
	_, e, _ := s.CreateKey("admin", "acme", nil, 0)
	got, ok := s.LookupByID(e.KeyID)
	if !ok {
		t.Fatal("lookup failed")
	}
	if got.Identity != "admin" {
		t.Fatal("identity mismatch")
	}
}

func TestAdminStore_LookupByID_Missing(t *testing.T) {
	s := NewAdminStore()
	if _, ok := s.LookupByID("missing"); ok {
		t.Fatal("expected miss")
	}
}

func TestAdminStore_Rotate(t *testing.T) {
	s := NewAdminStore()
	raw1, e1, _ := s.CreateKey("admin", "acme", []string{"rules:read"}, 0)
	raw2, old, e2, err := s.RotateKey(e1.KeyID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if raw1 == raw2 {
		t.Fatal("rotated token should differ")
	}
	if e2.KeyID == e1.KeyID {
		t.Fatal("new KeyID should differ")
	}
	if e2.RotatedFrom != e1.KeyID {
		t.Fatal("RotatedFrom should reference old key")
	}
	if !old.ExpiresAt.After(time.Now()) {
		t.Fatal("old key should have an expiry in the future")
	}
	// Both tokens still resolve during the grace window.
	if _, ok := s.LookupByToken(raw1); !ok {
		t.Fatal("old token should still work during grace")
	}
	if _, ok := s.LookupByToken(raw2); !ok {
		t.Fatal("new token should work")
	}
}

func TestAdminStore_Rotate_Missing(t *testing.T) {
	s := NewAdminStore()
	if _, _, _, err := s.RotateKey("missing", 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdminStore_Disable(t *testing.T) {
	s := NewAdminStore()
	raw, e, _ := s.CreateKey("admin", "acme", nil, 0)
	if err := s.DisableKey(e.KeyID); err != nil {
		t.Fatal(err)
	}
	got, ok := s.LookupByToken(raw)
	if !ok {
		t.Fatal("disabled key still resolves for hash lookup")
	}
	if !got.Disabled {
		t.Fatal("expected Disabled=true")
	}
}

func TestAdminStore_Disable_Missing(t *testing.T) {
	s := NewAdminStore()
	if err := s.DisableKey("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdminStore_Delete(t *testing.T) {
	s := NewAdminStore()
	raw, e, _ := s.CreateKey("admin", "acme", nil, 0)
	if err := s.DeleteKey(e.KeyID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.LookupByToken(raw); ok {
		t.Fatal("token should be gone")
	}
}

func TestAdminStore_Delete_Missing(t *testing.T) {
	s := NewAdminStore()
	if err := s.DeleteKey("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdminStore_List(t *testing.T) {
	s := NewAdminStore()
	for i := 0; i < 5; i++ {
		s.CreateKey("admin", "acme", nil, 0)
	}
	list := s.ListKeys()
	if len(list) != 5 {
		t.Fatalf("expected 5, got %d", len(list))
	}
}

func TestAdminStore_PurgeExpired(t *testing.T) {
	s := NewAdminStore()
	// Create a key already expired by setting CreatedAt manually.
	raw, e, _ := s.CreateKey("admin", "acme", nil, 0)
	e.ExpiresAt = time.Now().Add(-time.Hour)
	// And a fresh one.
	_, _, _ = s.CreateKey("admin", "acme", nil, time.Hour)
	n := s.PurgeExpired(time.Now())
	if n != 1 {
		t.Fatalf("purged: %d", n)
	}
	if _, ok := s.LookupByToken(raw); ok {
		t.Fatal("expired key should be purged")
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	t1, _ := GenerateToken()
	t2, _ := GenerateToken()
	if t1 == t2 {
		t.Fatal("tokens should be unique")
	}
	if len(t1) != 64 {
		t.Fatalf("token length: %d", len(t1))
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	a := hashToken("hello")
	b := hashToken("hello")
	if a != b {
		t.Fatal("hash should be deterministic")
	}
	if !strings.HasPrefix(a, "2cf24") {
		t.Fatal("expected known sha256(\"hello\")")
	}
}
