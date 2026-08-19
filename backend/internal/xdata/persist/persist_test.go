package persist

import (
	"path/filepath"
	"testing"
)

func TestPutGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Put("a", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	v, ok := d.Get("a")
	if !ok || string(v) != "hello" {
		t.Fatal("get")
	}
}

func TestReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.db")
	d, _ := Open(path)
	d.Put("k", []byte("v"))
	d.Close()
	d2, _ := Open(path)
	defer d2.Close()
	v, ok := d2.Get("k")
	if !ok || string(v) != "v" {
		t.Fatal("reload")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	d, _ := Open(filepath.Join(dir, "x"))
	defer d.Close()
	d.Put("a", []byte("1"))
	d.Delete("a")
	if d.Has("a") {
		t.Fatal("del")
	}
}

func TestLen(t *testing.T) {
	dir := t.TempDir()
	d, _ := Open(filepath.Join(dir, "x"))
	defer d.Close()
	if d.Len() != 0 {
		t.Fatal("empty")
	}
	d.Put("a", []byte("1"))
	if d.Len() != 1 {
		t.Fatal("len")
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	d, _ := Open(p)
	defer d.Close()
	if d.Path() != p {
		t.Fatal("path")
	}
}
