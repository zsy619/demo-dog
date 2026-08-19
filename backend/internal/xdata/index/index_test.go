package index

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPutGet(t *testing.T) {
	tr := New()
	tr.Put([]byte("a"), 1)
	tr.Put([]byte("b"), 2)
	v, ok := tr.Get([]byte("a"))
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestPut_Update(t *testing.T) {
	tr := New()
	tr.Put([]byte("a"), 1)
	tr.Put([]byte("a"), 2)
	v, _ := tr.Get([]byte("a"))
	if v.(int) != 2 {
		t.Fatal("update")
	}
}

func TestGet_Missing(t *testing.T) {
	tr := New()
	if _, ok := tr.Get([]byte("x")); ok {
		t.Fatal("missing")
	}
}

func TestDelete(t *testing.T) {
	tr := New()
	tr.Put([]byte("a"), 1)
	if !tr.Delete([]byte("a")) {
		t.Fatal("delete")
	}
	if _, ok := tr.Get([]byte("a")); ok {
		t.Fatal("应不存在")
	}
}

func TestDelete_Missing(t *testing.T) {
	tr := New()
	if tr.Delete([]byte("x")) {
		t.Fatal("应不命中")
	}
}

func TestRange(t *testing.T) {
	tr := New()
	for i := 0; i < 10; i++ {
		tr.Put([]byte(fmt.Sprintf("k%02d", i)), i)
	}
	out := tr.Range([]byte("k02"), []byte("k05"))
	if len(out) != 3 {
		t.Fatal("range")
	}
}

func TestMany(t *testing.T) {
	tr := New()
	for i := 0; i < 1000; i++ {
		tr.Put([]byte(fmt.Sprintf("k%04d", i)), i)
	}
	if tr.Len() != 1000 {
		t.Fatal("len")
	}
	if _, ok := tr.Get([]byte("k0500")); !ok {
		t.Fatal("get")
	}
}

func TestCompare(t *testing.T) {
	if compare([]byte("abc"), []byte("abd")) != -1 {
		t.Fatal("compare")
	}
	if !bytes.Equal([]byte("x"), []byte("x")) {
		t.Fatal("eq")
	}
}
