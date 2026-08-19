package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok"))
	}))
}

func TestGet(t *testing.T) {
	s := newServer()
	defer s.Close()
	c := New(Config{Timeout: time.Second})
	r, err := c.Get(context.Background(), s.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Body) != "ok" {
		t.Fatal("body")
	}
}

func TestPost(t *testing.T) {
	s := newServer()
	defer s.Close()
	c := New(Config{Timeout: time.Second})
	r, err := c.Post(context.Background(), s.URL+"/", []byte("x"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Body) != "ok" {
		t.Fatal("post")
	}
}

func TestExpectStatus(t *testing.T) {
	r := &Response{Status: 200}
	if err := r.ExpectStatus(); err != nil {
		t.Fatal("应通过")
	}
	r2 := &Response{Status: 500}
	if !errors.Is(r2.ExpectStatus(), ErrStatus) {
		t.Fatal("应 ErrStatus")
	}
}

func TestRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "no", 500)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := New(Config{Timeout: time.Second, Retry: 5, Backoff: time.Millisecond})
	r, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatal("重试次数:", attempts)
	}
	if string(r.Body) != "ok" {
		t.Fatal("body")
	}
}

func TestContextCancel(t *testing.T) {
	c := New(Config{Timeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Get(ctx, "http://127.0.0.1:1"); err == nil {
		t.Fatal("应取消")
	}
}
