package lrukv

import (
	"bytes"
	"testing"
)

func TestPutGet(t *testing.T) {
	k := New(8)
	k.Put("a", []byte("1"))
	v, ok := k.Get("a")
	if !ok || !bytes.Equal(v, []byte("1")) {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	k := New(2)
	k.Put("a", []byte("1"))
	k.Put("b", []byte("2"))
	k.Put("c", []byte("3"))
	if _, ok := k.Get("a"); ok {
		t.Fatal("evict")
	}
}

func TestLRUOrder(t *testing.T) {
	k := New(2)
	k.Put("a", []byte("1"))
	k.Put("b", []byte("2"))
	k.Get("a")
	k.Put("c", []byte("3"))
	if _, ok := k.Get("b"); ok {
		t.Fatal("b 应被驱逐")
	}
}

func TestKeys(t *testing.T) {
	k := New(4)
	k.Put("a", []byte("1"))
	k.Put("b", []byte("2"))
	ks := k.Keys()
	if len(ks) != 2 || ks[0] != "b" {
		t.Fatal("keys", ks)
	}
}

func TestUpdate(t *testing.T) {
	k := New(4)
	k.Put("a", []byte("1"))
	k.Put("a", []byte("2"))
	v, _ := k.Get("a")
	if !bytes.Equal(v, []byte("2")) {
		t.Fatal("upd")
	}
}
