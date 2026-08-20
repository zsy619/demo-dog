package health

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAggregator_RegisterAndRun(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{Name: "ok", Probe: func(ctx context.Context) error { return nil }})
	a.Register(&Check{Name: "bad", Probe: func(ctx context.Context) error { return errors.New("nope") }})
	snap := a.RunAll(context.Background())
	if snap.Healthy {
		t.Fatal("should not be healthy")
	}
	if snap.OK != 1 || snap.Failed != 1 {
		t.Fatalf("counts: %+v", snap)
	}
	if snap.Items["ok"].Status != "ok" {
		t.Fatal("ok status")
	}
	if snap.Items["bad"].Error != "nope" {
		t.Fatal("bad error")
	}
}

func TestAggregator_AllHealthy(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{Name: "a", Probe: func(ctx context.Context) error { return nil }})
	a.Register(&Check{Name: "b", Probe: func(ctx context.Context) error { return nil }})
	snap := a.RunAll(context.Background())
	if !snap.Healthy {
		t.Fatal("expected healthy")
	}
}

func TestAggregator_Remove(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{Name: "a", Probe: func(ctx context.Context) error { return nil }})
	a.Remove("a")
	snap := a.RunAll(context.Background())
	if len(snap.Items) != 0 {
		t.Fatal("expected empty")
	}
}

func TestAggregator_CriticalFailure(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{Name: "a", Critical: true, Probe: func(ctx context.Context) error { return errors.New("x") }})
	a.Register(&Check{Name: "b", Probe: func(ctx context.Context) error { return nil }})
	snap := a.RunAll(context.Background())
	if snap.Healthy {
		t.Fatal("critical failure should mark unhealthy")
	}
}

func TestAggregator_Timeout(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{
		Name: "slow",
		Threshold: 50 * time.Millisecond,
		Probe: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	snap := a.RunAll(context.Background())
	if snap.Items["slow"].Status != "failed" {
		t.Fatalf("status: %s", snap.Items["slow"].Status)
	}
}

func TestAggregator_NilProbe(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{Name: "x"})
	snap := a.RunAll(context.Background())
	if !snap.Healthy {
		t.Fatal("nil probe should pass")
	}
}

func TestHTTPCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := HTTPCheck("http", srv.URL, true)
	if err := c.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPCheck_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c := HTTPCheck("http", srv.URL, true)
	if err := c.Probe(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestTCPCheck(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c := TCPCheck("tcp", l.Addr().String(), true)
	if err := c.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTCPCheck_Fails(t *testing.T) {
	c := TCPCheck("tcp", "127.0.0.1:1", true)
	if err := c.Probe(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestWorkerCheck(t *testing.T) {
	c := WorkerCheck("pool", 5, 10, true)
	if err := c.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	c2 := WorkerCheck("pool", 20, 10, true)
	if err := c2.Probe(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestAggregator_HandleHTTP(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{Name: "ok", Probe: func(ctx context.Context) error { return nil }})
	h := a.HandleHTTP()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestAggregator_HandleHTTP_503(t *testing.T) {
	a := NewAggregator()
	a.Register(&Check{Name: "bad", Probe: func(ctx context.Context) error { return errors.New("x") }})
	h := a.HandleHTTP()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 503 {
		t.Fatalf("status: %d", rr.Code)
	}
}
