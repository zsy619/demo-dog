package reqlog

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := Wrap(w)
		rc.WriteHeader(201)
		rc.Write([]byte("hi"))
		if rc.Status() != 201 {
			t.Fatal("status")
		}
		if rc.Size() != 2 {
			t.Fatal("size")
		}
	}))
	defer ts.Close()
	http.Get(ts.URL)
}

func TestSnapshot(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := Wrap(w)
		rc.Write([]byte("hello"))
		log := rc.Snapshot(r)
		if log.Method != "GET" || log.Status != 200 || log.Size != 5 {
			t.Fatal("snap", log)
		}
	}))
	defer ts.Close()
	http.Get(ts.URL + "/x")
}

func TestHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := Wrap(w)
		rc.Header().Set("X-Test", "1")
		rc.Write([]byte("x"))
	}))
	defer ts.Close()
	r, _ := http.Get(ts.URL)
	if r.Header.Get("X-Test") != "1" {
		t.Fatal("hdr")
	}
}
