package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	body := []byte("hello world")
	sig := Sign(body, "secret")
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatal("missing prefix")
	}
	if !Verify(body, "secret", sig) {
		t.Fatal("verify failed")
	}
	if Verify(body, "wrong", sig) {
		t.Fatal("verify accepted wrong secret")
	}
	if Verify([]byte("different"), "secret", sig) {
		t.Fatal("verify accepted tampered body")
	}
}

func TestSubscriber_Accept(t *testing.T) {
	s := &Subscriber{}
	if !s.Accept("any") {
		t.Fatal("empty filter should accept all")
	}
	s.EventTypes = []string{"alert.fired", "alert.resolved"}
	if !s.Accept("alert.fired") {
		t.Fatal("should accept")
	}
	if s.Accept("rule.created") {
		t.Fatal("should reject")
	}
}

func TestSubscriber_Defaults(t *testing.T) {
	s := &Subscriber{}
	if s.timeout() != 5*time.Second {
		t.Fatal("default timeout")
	}
	if s.maxRetries() != 0 {
		t.Fatal("default retries")
	}
	s.MaxRetries = 100
	if s.maxRetries() != 10 {
		t.Fatal("max retries cap")
	}
	s.MaxRetries = -1
	if s.maxRetries() != 0 {
		t.Fatal("negative retries")
	}
}

func TestAddRemoveSubscriber(t *testing.T) {
	d := NewDispatcher(0)
	if err := d.AddSubscriber(&Subscriber{ID: "a", URL: "http://x"}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSubscriber(&Subscriber{ID: ""}); err == nil {
		t.Fatal("empty id should fail")
	}
	if err := d.AddSubscriber(&Subscriber{ID: "b"}); err == nil {
		t.Fatal("empty url should fail")
	}
	if got := len(d.Subscribers()); got != 1 {
		t.Fatalf("subscribers: %d", got)
	}
	d.RemoveSubscriber("a")
	if got := len(d.Subscribers()); got != 0 {
		t.Fatalf("subscribers after rm: %d", got)
	}
}

func TestDispatch_HappyPath(t *testing.T) {
	var got []byte
	var sig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		sig = r.Header.Get("X-DemoDog-Signature")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	d := NewDispatcher(0)
	d.AddSubscriber(&Subscriber{ID: "x", URL: srv.URL, Secret: "k"})
	dels := d.Dispatch(Event{Type: "alert.fired", Tenant: "acme", Payload: map[string]string{"a": "b"}})
	if len(dels) != 1 {
		t.Fatalf("deliveries: %d", len(dels))
	}
	if !dels[0].Success() {
		t.Fatalf("delivery failed: %s", dels[0].Error)
	}
	if dels[0].Status != 200 {
		t.Fatalf("status: %d", dels[0].Status)
	}
	if len(got) == 0 {
		t.Fatal("no body received")
	}
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatal("no signature")
	}
	var ev Event
	if err := json.Unmarshal(got, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "alert.fired" {
		t.Fatal("type mismatch")
	}
}

func TestDispatch_RetriesOnServerError(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	d := NewDispatcher(0)
	d.AddSubscriber(&Subscriber{ID: "x", URL: srv.URL, Secret: "k", MaxRetries: 5})
	dels := d.Dispatch(Event{Type: "alert.fired"})
	if !dels[0].Success() {
		t.Fatalf("expected success after retry: %s", dels[0].Error)
	}
	if dels[0].Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", dels[0].Attempts)
	}
}

func TestDispatch_PermanentFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	d := NewDispatcher(8)
	d.AddSubscriber(&Subscriber{ID: "x", URL: srv.URL, Secret: "k", MaxRetries: 2})
	dels := d.Dispatch(Event{Type: "alert.fired"})
	if dels[0].Success() {
		t.Fatal("expected failure")
	}
	if dels[0].Attempts != 3 {
		t.Fatalf("expected 3 attempts (1 + 2 retries), got %d", dels[0].Attempts)
	}
	if len(d.DeadLetters()) != 1 {
		t.Fatal("expected DLQ entry")
	}
	stats := d.Stats()
	if stats.Failed != 1 {
		t.Fatalf("failed counter: %d", stats.Failed)
	}
}

func TestDispatch_FilterByEventType(t *testing.T) {
	var got int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()
	d := NewDispatcher(0)
	d.AddSubscriber(&Subscriber{ID: "x", URL: srv.URL, EventTypes: []string{"alert.fired"}})
	d.Dispatch(Event{Type: "rule.created"})
	d.Dispatch(Event{Type: "alert.fired"})
	mu.Lock()
	defer mu.Unlock()
	if got != 1 {
		t.Fatalf("expected 1 hit, got %d", got)
	}
}

func TestDispatch_BadURL(t *testing.T) {
	d := NewDispatcher(0)
	d.AddSubscriber(&Subscriber{ID: "x", URL: "://broken", Secret: "k", MaxRetries: 0})
	dels := d.Dispatch(Event{Type: "alert.fired"})
	if dels[0].Success() {
		t.Fatal("expected failure")
	}
}

func TestDeadLetters_Ring(t *testing.T) {
	d := NewDispatcher(2)
	d1 := Delivery{EventID: "a"}
	d2 := Delivery{EventID: "b"}
	d3 := Delivery{EventID: "c"}
	d.recordDLQ(d1)
	d.recordDLQ(d2)
	d.recordDLQ(d3)
	dlq := d.DeadLetters()
	if len(dlq) != 2 {
		t.Fatalf("dlq size: %d", len(dlq))
	}
	if dlq[0].EventID != "b" || dlq[1].EventID != "c" {
		t.Fatalf("unexpected order: %+v", dlq)
	}
}

func TestStats(t *testing.T) {
	d := NewDispatcher(0)
	d.AddSubscriber(&Subscriber{ID: "x", URL: "http://x"})
	s := d.Stats()
	if s.Subscribers != 1 {
		t.Fatal("subscribers")
	}
}

func TestBackoff_Cap(t *testing.T) {
	d := backoff(20)
	if d > 30*time.Second {
		t.Fatalf("backoff not capped: %v", d)
	}
}
