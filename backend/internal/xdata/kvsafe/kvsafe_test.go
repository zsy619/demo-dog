package kvsafe

import (
	"bytes"
	"os"
	"testing"
)

func TestOpen_New(t *testing.T) {
	s, err := Open("/tmp/kvsafe_new.db", []byte("1234567890123456"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Len() != 0 {
		t.Fatal("空文件应为 0")
	}
}

func TestPutFlushGet(t *testing.T) {
	path := "/tmp/kvsafe_pf.db"
	os.Remove(path)
	s, err := Open(path, []byte("1234567890123456"))
	if err != nil {
		t.Fatal(err)
	}
	s.Put("a", []byte("hello"))
	s.Put("b", []byte("world"))
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path, []byte("1234567890123456"))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := s2.Get("a")
	if !ok || !bytes.Equal(v, []byte("hello")) {
		t.Fatal("回环不一致")
	}
}

func TestDelete(t *testing.T) {
	s, _ := Open("/tmp/kvsafe_del.db", []byte("1234567890123456"))
	s.Put("a", []byte("x"))
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("应已被删除")
	}
}

func TestSnapshot(t *testing.T) {
	s, _ := Open("/tmp/kvsafe_snap.db", []byte("1234567890123456"))
	s.Put("a", []byte("x"))
	s.Put("b", []byte("y"))
	if len(s.Snapshot()) != 2 {
		t.Fatal("应有两个键")
	}
}

func TestFlushNoDirty(t *testing.T) {
	s, _ := Open("/tmp/kvsafe_nodirty.db", []byte("1234567890123456"))
	if err := s.Flush(); err != nil {
		t.Fatal("应无操作")
	}
}

func TestBadKey(t *testing.T) {
	os.Remove("/tmp/kvsafe_bad.db")
	s, err := Open("/tmp/kvsafe_bad.db", []byte("short"))
	if err != nil {
		return // Open may succeed; Flush is what fails
	}
	if err := s.Flush(); err == nil {
		t.Fatal("短 key Flush 应报错")
	}
}
