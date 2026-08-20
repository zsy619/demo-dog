package filewatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatch_Modified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("v1"), 0644)
	w := New(20 * time.Millisecond)
	w.Watch(path)
	w.Start()
	defer w.Stop()
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(path, []byte("v2 long content"), 0644)
	select {
	case ev := <-w.Events:
		if ev.Path != path {
			t.Fatal("路径不匹配")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("超时未收到事件")
	}
}

func TestWatch_Created(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.txt")
	w := New(20 * time.Millisecond)
	w.Watch(path)
	w.Start()
	defer w.Stop()
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(path, []byte("x"), 0644)
	select {
	case ev := <-w.Events:
		if ev.Kind != EventCreated {
			t.Fatal("应为 Created")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("超时")
	}
}

func TestWatch_Deleted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.txt")
	os.WriteFile(path, []byte("y"), 0644)
	w := New(20 * time.Millisecond)
	w.Watch(path)
	w.Start()
	defer w.Stop()
	time.Sleep(50 * time.Millisecond)
	os.Remove(path)
	select {
	case ev := <-w.Events:
		if ev.Kind != EventDeleted {
			t.Fatal("应为 Deleted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("超时")
	}
}

func TestUnwatch(t *testing.T) {
	w := New(20 * time.Millisecond)
	w.Watch("/tmp/x")
	w.Unwatch("/tmp/x")
	w.Stop()
}

func TestEmptyPath(t *testing.T) {
	w := New(time.Second)
	if err := w.Watch(""); err != ErrEmptyPath {
		t.Fatal("应报错")
	}
}
