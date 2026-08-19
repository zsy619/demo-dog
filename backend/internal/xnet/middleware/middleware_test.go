package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func hello(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("hi"))
}

func TestChain(t *testing.T) {
	h := Chain(http.HandlerFunc(hello), CacheControl("no-cache"))
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Result().Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatal("cache")
	}
}

func TestRecover(t *testing.T) {
	bang := func(_ http.ResponseWriter, _ *http.Request) { panic("oops") }
	h := Chain(http.HandlerFunc(bang), Recover)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 500 {
		t.Fatal("应 500")
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	h := Chain(http.HandlerFunc(hello), Logger(log.New(&buf, "", 0)))
	r := httptest.NewRequest("GET", "/a", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !strings.Contains(buf.String(), "GET /a") {
		t.Fatal("log")
	}
}

func TestTimeout(t *testing.T) {
	slow := func(_ http.ResponseWriter, _ *http.Request) { time.Sleep(100 * time.Millisecond) }
	h := Chain(http.HandlerFunc(slow), Timeout(20*time.Millisecond))
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 504 {
		t.Fatal("应 504")
	}
}

func TestCORS_Options(t *testing.T) {
	h := Chain(http.HandlerFunc(hello), CORS("*"))
	r := httptest.NewRequest("OPTIONS", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal("应 204")
	}
	if w.Result().Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("应设置头")
	}
}
