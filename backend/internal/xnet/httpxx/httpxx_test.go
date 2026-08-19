package httpxx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()
	c := New()
	c.SetBaseURL(ts.URL)
	r, err := c.Get(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatal("status")
	}
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"echo": "ok"})
	}))
	defer ts.Close()
	c := New()
	c.SetBaseURL(ts.URL)
	r, err := c.Post(context.Background(), "/", map[string]string{"x": "y"})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]string
	if err := DoJSON(r, &out); err != nil {
		t.Fatal(err)
	}
	if out["echo"] != "ok" {
		t.Fatal("echo")
	}
}

func TestSetTimeout(t *testing.T) {
	c := New()
	c.SetTimeout(50 * time.Millisecond)
	if c.timeout != 50*time.Millisecond {
		t.Fatal("to")
	}
}

func TestSetHeader(t *testing.T) {
	c := New()
	c.SetHeader("X-Test", "1")
	if c.headers["X-Test"] != "1" {
		t.Fatal("hdr")
	}
}

func TestDoJSON_Nil(t *testing.T) {
	if err := DoJSON(nil, nil); err == nil {
		t.Fatal("应报错")
	}
}

func TestDoJSON_BadStatus(t *testing.T) {
	r, _ := http.Get("http://nonexistent.example.invalid/")
	if r != nil {
		defer r.Body.Close()
		type t2 struct{}
		_ = DoJSON(r, &t2{})
	}
}
