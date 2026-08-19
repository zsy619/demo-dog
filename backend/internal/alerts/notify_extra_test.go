package alerts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSlackChannel_Post(t *testing.T) {
	var got bytes.Buffer
	var ct string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		io.Copy(&got, r.Body)
		w.WriteHeader(200)
	}))
	defer ts.Close()
	s := &SlackChannel{WebhookURL: ts.URL, Channel: "#alerts", Username: "demo-dog"}
	if err := s.Send(context.Background(), NotifyOpts{
		Subject:  "p99 above 1s",
		Body:     "checkout at 1.4s p99",
		Severity: "error",
		Labels:   map[string]string{"service": "checkout"},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("ct: %s", ct)
	}
	body := got.String()
	for _, want := range []string{"p99 above 1s", "checkout", "ERROR"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in payload: %s", want, body)
		}
	}
}

func TestSlackChannel_Empty(t *testing.T) {
	s := &SlackChannel{}
	if err := s.Send(context.Background(), NotifyOpts{}); err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestSlackChannel_5xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal"))
	}))
	defer ts.Close()
	s := &SlackChannel{WebhookURL: ts.URL}
	err := s.Send(context.Background(), NotifyOpts{Subject: "x", Severity: "info"})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("err: %v", err)
	}
}

type fakeChannel struct {
	calls atomic.Int64
	returnErr error
	kind  ChannelKind
}

func (f *fakeChannel) Kind() ChannelKind { return f.kind }
func (f *fakeChannel) Send(_ context.Context, _ NotifyOpts) error {
	f.calls.Add(1)
	return f.returnErr
}

// ChannelFunc lets tests wrap a function as a Channel.
type ChannelFunc func(ctx context.Context, opts NotifyOpts) error

func (f ChannelFunc) Kind() ChannelKind { return ChannelWebhook }
func (f ChannelFunc) Send(ctx context.Context, opts NotifyOpts) error {
	return f(ctx, opts)
}

func TestRetryChannel_Success(t *testing.T) {
	f := &fakeChannel{kind: ChannelWebhook, returnErr: nil}
	r := &RetryChannel{Inner: f, Attempts: 3, BaseWait: 1 * time.Millisecond}
	if err := r.Send(context.Background(), NotifyOpts{}); err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 1 {
		t.Fatalf("calls: %d", f.calls.Load())
	}
}

func TestRetryChannel_RetriesUntilSuccess(t *testing.T) {
	// Custom inner that fails N times then succeeds.
	var attempts atomic.Int64
	inner := ChannelFunc(func(_ context.Context, _ NotifyOpts) error {
		n := attempts.Add(1)
		if n < 3 {
			return errors.New("transient")
		}
		return nil
	})
	r := &RetryChannel{Inner: inner, Attempts: 5, BaseWait: 1 * time.Millisecond}
	if err := r.Send(context.Background(), NotifyOpts{}); err != nil {
		t.Fatalf("expected eventual success: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts: %d", attempts.Load())
	}
}

func TestRetryChannel_Exhaust(t *testing.T) {
	inner := ChannelFunc(func(_ context.Context, _ NotifyOpts) error {
		return errors.New("boom")
	})
	r := &RetryChannel{Inner: inner, Attempts: 3, BaseWait: 1 * time.Millisecond}
	err := r.Send(context.Background(), NotifyOpts{})
	if err == nil {
		t.Fatal("expected error after exhaustion")
	}
	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Fatalf("err: %v", err)
	}
}

func TestRetryChannel_DefaultsApplied(t *testing.T) {
	var calls atomic.Int64
	inner := ChannelFunc(func(_ context.Context, _ NotifyOpts) error {
		calls.Add(1)
		return errors.New("x")
	})
	r := &RetryChannel{Inner: inner}
	err := r.Send(context.Background(), NotifyOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 default attempts, got %d", calls.Load())
	}
}

func TestSeverityColor(t *testing.T) {
	cases := map[string]string{
		"fatal":   "#dc3545",
		"error":   "#fd7e14",
		"warn":    "#ffc107",
		"info":    "#0dcaf0",
		"unknown": "#6c757d",
	}
	for sev, want := range cases {
		if got := SeverityColor(sev); got != want {
			t.Errorf("%s: got %s want %s", sev, got, want)
		}
	}
}
