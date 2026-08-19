package registry

import (
	"errors"
	"testing"
)

func TestSetGet(t *testing.T) {
	r := New()
	r.Set("k", "v", "init")
	v, ok := r.Get("k")
	if !ok || v != "v" {
		t.Fatal("get")
	}
	if r.Reason("k") != "init" {
		t.Fatal("reason")
	}
}

func TestGetString(t *testing.T) {
	r := New()
	r.Set("k", "v", "")
	v, err := r.GetString("k")
	if err != nil || v != "v" {
		t.Fatal(err)
	}
}

func TestGetString_Missing(t *testing.T) {
	r := New()
	if _, err := r.GetString("missing"); !errors.Is(err, ErrKeyMissing) {
		t.Fatal(err)
	}
}

func TestGetString_BadType(t *testing.T) {
	r := New()
	r.Set("k", 1, "")
	if _, err := r.GetString("k"); !errors.Is(err, ErrBadType) {
		t.Fatal(err)
	}
}

func TestGetInt(t *testing.T) {
	r := New()
	r.Set("k", 42, "")
	v, err := r.GetInt("k")
	if err != nil || v != 42 {
		t.Fatal(err)
	}
}

func TestGetInt_Float(t *testing.T) {
	r := New()
	r.Set("k", float64(42), "")
	v, err := r.GetInt("k")
	if err != nil || v != 42 {
		t.Fatal(err)
	}
}

func TestGetInt_Int64(t *testing.T) {
	r := New()
	r.Set("k", int64(42), "")
	v, err := r.GetInt("k")
	if err != nil || v != 42 {
		t.Fatal(err)
	}
}

func TestGetBool(t *testing.T) {
	r := New()
	r.Set("k", true, "")
	v, err := r.GetBool("k")
	if err != nil || !v {
		t.Fatal(err)
	}
}

func TestGetBool_Missing(t *testing.T) {
	r := New()
	if _, err := r.GetBool("missing"); !errors.Is(err, ErrKeyMissing) {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	r := New()
	r.Set("k", "v", "")
	r.Delete("k")
	if _, ok := r.Get("k"); ok {
		t.Fatal("delete")
	}
}

func TestSnapshot(t *testing.T) {
	r := New()
	r.Set("a", 1, "")
	r.Set("b", 2, "")
	s := r.Snapshot()
	if len(s) != 2 {
		t.Fatal("snapshot")
	}
	if s["a"] != 1 || s["b"] != 2 {
		t.Fatal("values")
	}
}

func TestVersion(t *testing.T) {
	r := New()
	v1 := r.Version()
	r.Set("k", 1, "")
	if r.Version() <= v1 {
		t.Fatal("version should increase")
	}
}

func TestKeys(t *testing.T) {
	r := New()
	r.Set("b", 1, "")
	r.Set("a", 2, "")
	keys := r.Keys()
	if len(keys) != 2 {
		t.Fatal("keys")
	}
}

func TestGet_Missing(t *testing.T) {
	r := New()
	if _, ok := r.Get("missing"); ok {
		t.Fatal("missing")
	}
}

func TestReason_Missing(t *testing.T) {
	r := New()
	if r.Reason("missing") != "" {
		t.Fatal("missing reason")
	}
}
