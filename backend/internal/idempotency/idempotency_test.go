package idempotency

import (
	"net/http"
	"testing"
	"time"
)

func TestStore_SaveLookup(t *testing.T) {
	s := New(time.Minute, 0)
	s.Save("k1", 200, []byte("hello"), http.Header{"X-Foo": {"bar"}})
	r := s.Lookup("k1")
	if r == nil {
		t.Fatal("expected record")
	}
	if r.Status != 200 || string(r.Body) != "hello" {
		t.Fatal("record fields")
	}
	if r.Header.Get("X-Foo") != "bar" {
		t.Fatal("header")
	}
}

func TestStore_LookupMiss(t *testing.T) {
	s := New(time.Minute, 0)
	if r := s.Lookup("missing"); r != nil {
		t.Fatal("expected nil")
	}
}

func TestStore_TTLExpiry(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(50*time.Millisecond, 0).WithTime(func() time.Time { return now })
	s.Save("k", 200, []byte("x"), nil)
	now = now.Add(100 * time.Millisecond)
	if r := s.Lookup("k"); r != nil {
		t.Fatal("expected expiry")
	}
}

func TestStore_EvictOnMaxItems(t *testing.T) {
	s := New(time.Minute, 2)
	s.Save("a", 200, []byte("1"), nil)
	time.Sleep(time.Millisecond)
	s.Save("b", 200, []byte("2"), nil)
	time.Sleep(time.Millisecond)
	s.Save("c", 200, []byte("3"), nil)
	if s.Len() > 2 {
		t.Fatal("len")
	}
}

func TestStore_Forget(t *testing.T) {
	s := New(time.Minute, 0)
	s.Save("k", 200, []byte("x"), nil)
	s.Forget("k")
	if r := s.Lookup("k"); r != nil {
		t.Fatal("forget")
	}
}

func TestHashRequest(t *testing.T) {
	a := HashRequest([]byte("hello"))
	b := HashRequest([]byte("hello"))
	c := HashRequest([]byte("world"))
	if a != b || a == c {
		t.Fatal("hash")
	}
	if len(a) != 64 {
		t.Fatal("hex length")
	}
}

func TestMiddleware_LookupNoKey(t *testing.T) {
	s := New(time.Minute, 0)
	mw := &Middleware{Store: s}
	r := mustReq("/")
	if _, err := mw.Lookup(r); err != nil {
		t.Fatal(err)
	}
}

func TestMiddleware_LookupRequireKeyMissing(t *testing.T) {
	s := New(time.Minute, 0)
	mw := &Middleware{Store: s, RequireKey: true}
	r := mustReq("/")
	if _, err := mw.Lookup(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestMiddleware_LookupHit(t *testing.T) {
	s := New(time.Minute, 0)
	s.Save("k", 201, []byte("ok"), nil)
	mw := &Middleware{Store: s}
	r := mustReq("/")
	r.Header.Set("Idempotency-Key", "k")
	rec, err := mw.Lookup(r)
	if err != nil || rec == nil {
		t.Fatal(err)
	}
	if rec.Status != 201 {
		t.Fatal("status")
	}
}

func mustReq(path string) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, path, nil)
	return req
}
