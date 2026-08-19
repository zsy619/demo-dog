package pprofx

import (
	"io"
	"net/http"
	"testing"
	"time"
)

func TestStartStop(t *testing.T) {
	s := New()
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	if !s.Running() {
		t.Fatal("未运行")
	}
	addr := s.Addr()
	resp, err := http.Get("http://" + addr + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != 200 {
		t.Fatal("status")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestDoubleStart(t *testing.T) {
	s := New()
	s.Start("127.0.0.1:0")
	defer s.Stop()
	if err := s.Start("127.0.0.1:0"); err == nil {
		t.Fatal("应报错")
	}
}

func TestStopIdempotent(t *testing.T) {
	s := New()
	if err := s.Stop(); err != nil {
		t.Fatal("未启动 Stop 应 nil")
	}
}

func TestHandler(t *testing.T) {
	h := Handler()
	ts := &http.Server{Handler: h, Addr: "127.0.0.1:0"}
	go ts.ListenAndServe()
	time.Sleep(50 * time.Millisecond)
	ts.Shutdown(nil)
}
