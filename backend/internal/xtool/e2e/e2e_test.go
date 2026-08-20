package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ping":
			w.Write([]byte("pong"))
		case "/echo":
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			w.Write(buf[:n])
		case "/method":
			w.Write([]byte(r.Method))
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestGet(t *testing.T) {
	srv := newServer()
	c := New(srv)
	defer c.Close()
	resp, err := c.Get("/ping")
	if err != nil {
		t.Fatal(err)
	}
	resp.ExpectStatus(t, 200)
	resp.ExpectBody(t, "pong")
}

func TestPost(t *testing.T) {
	srv := newServer()
	c := New(srv)
	defer c.Close()
	resp, err := c.Do("POST", "/echo", []byte("hello"), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.ExpectBody(t, "hello")
}

func TestPostJSON(t *testing.T) {
	srv := newServer()
	c := New(srv)
	defer c.Close()
	resp, err := c.PostJSON("/json", map[string]any{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	resp.DecodeJSON(t, &v)
	if v["ok"] != true {
		t.Fatal("json")
	}
}

func TestNotFound(t *testing.T) {
	srv := newServer()
	c := New(srv)
	defer c.Close()
	resp, _ := c.Get("/missing")
	resp.ExpectStatus(t, 404)
}

func TestMethod(t *testing.T) {
	srv := newServer()
	c := New(srv)
	defer c.Close()
	resp, _ := c.Do("DELETE", "/method", nil, nil)
	resp.ExpectBody(t, "DELETE")
}

func TestClose(t *testing.T) {
	srv := newServer()
	c := New(srv)
	c.Close()
	if _, err := c.Get("/ping"); err == nil {
		t.Fatal("关闭后应报错")
	}
}
