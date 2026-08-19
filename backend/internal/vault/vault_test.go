package vault

import (
	"bytes"
	"testing"
	"time"
)

func newTestVault() *Vault {
	master := bytes.Repeat([]byte("x"), 32)
	return NewVault(master, 0).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestPutGetRoundTrip(t *testing.T) {
	v := newTestVault()
	ver, err := v.Put("acme", "api_key", "sk-1234567890", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if ver != 1 {
		t.Fatal("version")
	}
	got, ok, err := v.Get("acme", "api_key", "alice")
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	if got != "sk-1234567890" {
		t.Fatalf("got %q", got)
	}
}

func TestPut_BumpsVersion(t *testing.T) {
	v := newTestVault()
	v1, _ := v.Put("a", "k", "one", "u")
	v2, _ := v.Put("a", "k", "two", "u")
	if v1 != 1 || v2 != 2 {
		t.Fatal("versions")
	}
	got, _, _ := v.Get("a", "k", "u")
	if got != "two" {
		t.Fatal("should be latest")
	}
}

func TestPut_TenantIsolation(t *testing.T) {
	v := newTestVault()
	v.Put("acme", "k", "acme-secret", "u")
	v.Put("globex", "k", "globex-secret", "u")
	a, _, _ := v.Get("acme", "k", "u")
	g, _, _ := v.Get("globex", "k", "u")
	if a != "acme-secret" || g != "globex-secret" {
		t.Fatal("isolation")
	}
}

func TestGet_Missing(t *testing.T) {
	v := newTestVault()
	_, ok, err := v.Get("missing", "k", "u")
	if err != nil || ok {
		t.Fatal("expected miss")
	}
}

func TestPut_RequiresFields(t *testing.T) {
	v := newTestVault()
	if _, err := v.Put("", "k", "x", "u"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := v.Put("a", "", "x", "u"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDelete(t *testing.T) {
	v := newTestVault()
	v.Put("a", "k", "x", "u")
	if err := v.Delete("a", "k", "u"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ := v.Get("a", "k", "u")
	if ok {
		t.Fatal("should be deleted")
	}
}

func TestDelete_Idempotent(t *testing.T) {
	v := newTestVault()
	if err := v.Delete("missing", "k", "u"); err != nil {
		t.Fatal("delete on missing should be no-op")
	}
}

func TestNames(t *testing.T) {
	v := newTestVault()
	v.Put("a", "k1", "x", "u")
	v.Put("a", "k2", "x", "u")
	v.Put("b", "k1", "x", "u")
	n := v.Names("a")
	if len(n) != 2 {
		t.Fatalf("names: %v", n)
	}
	if len(v.Names("missing")) != 0 {
		t.Fatal("missing should be empty")
	}
}

func TestAudit(t *testing.T) {
	v := newTestVault()
	v.Put("a", "k", "x", "alice")
	v.Get("a", "k", "bob")
	v.Get("missing", "k", "bob")
	v.Delete("a", "k", "alice")
	a := v.Audit()
	if len(a) != 4 {
		t.Fatalf("audit: %d", len(a))
	}
	if a[0].Action != "put" || !a[0].Found {
		t.Fatal("put")
	}
	if a[2].Found {
		t.Fatal("miss should not be Found")
	}
}

func TestAuditCap(t *testing.T) {
	v := NewVault(bytes.Repeat([]byte("x"), 32), 3)
	for i := 0; i < 10; i++ {
		v.Get("a", "k", "u")
	}
	if len(v.Audit()) != 3 {
		t.Fatal("cap")
	}
}

func TestNewVault_PanicsShortKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	NewVault([]byte("short"), 0)
}

func TestEncryptDecrypt(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	ct, nonce, err := encrypt(key, "hello")
	if err != nil {
		t.Fatal(err)
	}
	pt, err := decrypt(key, ct, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != "hello" {
		t.Fatal(pt)
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	key1 := bytes.Repeat([]byte("a"), 32)
	key2 := bytes.Repeat([]byte("b"), 32)
	ct, nonce, _ := encrypt(key1, "hello")
	if _, err := decrypt(key2, ct, nonce); err == nil {
		t.Fatal("expected decrypt failure")
	}
}

func TestEncodeAudit(t *testing.T) {
	e := EncodeAudit(AuditEntry{Tenant: "t", Name: "n", Actor: "a", Action: "get", At: time.Unix(1700000000, 0), Found: true})
	if e == "" {
		t.Fatal("empty")
	}
}
