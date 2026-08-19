package alerts

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecordingEngine_AddDefaultInterval(t *testing.T) {
	e := NewRecordingEngine()
	r := e.Add(RecordingRule{Name: "foo", Evaluate: func(_ context.Context) (float64, error) { return 0, nil }})
	if r != nil {
		t.Fatal("expected nil for first add")
	}
	e.mu.RLock()
	got := e.rules["foo"]
	e.mu.RUnlock()
	if got.rule.Interval != 30*time.Second {
		t.Fatalf("default interval: %v", got.rule.Interval)
	}
	if got.rule.NewMetric != "foo" {
		t.Fatalf("default new metric: %s", got.rule.NewMetric)
	}
}

func TestRecordingEngine_AddReplaces(t *testing.T) {
	e := NewRecordingEngine()
	e.Add(RecordingRule{Name: "foo", NewMetric: "foo_v1", Evaluate: nil})
	e.Add(RecordingRule{Name: "foo", NewMetric: "foo_v2", Evaluate: nil})
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.rules["foo"].rule.NewMetric != "foo_v2" {
		t.Fatal("replacement should win")
	}
	if len(e.rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(e.rules))
	}
}

func TestRecordingEngine_Remove(t *testing.T) {
	e := NewRecordingEngine()
	e.Add(RecordingRule{Name: "foo", Evaluate: nil})
	if !e.Remove("foo") {
		t.Fatal("Remove should return true for existing rule")
	}
	if e.Remove("foo") {
		t.Fatal("Remove should return false for missing rule")
	}
}

func TestRecordingEngine_RunSuccess(t *testing.T) {
	e := NewRecordingEngine()
	var calls atomic.Int64
	var persisted atomic.Int64
	e.Add(RecordingRule{
		Name:      "rate5m",
		NewMetric: "http_requests_5m",
		Interval:  20 * time.Millisecond,
		Evaluate: func(_ context.Context) (float64, error) {
			calls.Add(1)
			return 42, nil
		},
	})
	e.Persist = func(_ context.Context, _ RecordingResult) { persisted.Add(1) }
	ctx, cancel := context.WithCancel(context.Background())
	e.Start(ctx)
	defer func() { cancel(); e.Stop() }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected >= 2 calls, got %d", calls.Load())
	}
	if persisted.Load() == 0 {
		t.Fatal("expected persist to fire)")
	}
	snap := e.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot len: %d", len(snap))
	}
	if snap[0].LastValue != 42 {
		t.Fatalf("last value: %v", snap[0].LastValue)
	}
	if snap[0].LastError != "" {
		t.Fatalf("last error: %s", snap[0].LastError)
	}
}

func TestRecordingEngine_RunFailure(t *testing.T) {
	e := NewRecordingEngine()
	var calls atomic.Int64
	myErr := errors.New("compute failed")
	e.Add(RecordingRule{
		Name:     "broken",
		Interval: 20 * time.Millisecond,
		Evaluate: func(_ context.Context) (float64, error) {
			calls.Add(1)
			return 0, myErr
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	e.Start(ctx)
	defer func() { cancel(); e.Stop() }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	snap := e.Snapshot()
	if snap[0].Fails < 1 {
		t.Fatalf("expected fails >= 1, got %d", snap[0].Fails)
	}
	if snap[0].LastError != "compute failed" {
		t.Fatalf("last error: %s", snap[0].LastError)
	}
}

func TestRecordingEngine_StopClean(t *testing.T) {
	e := NewRecordingEngine()
	e.Add(RecordingRule{Name: "x", Interval: 20 * time.Millisecond,
		Evaluate: func(_ context.Context) (float64, error) { return 1, nil }})
	ctx, cancel := context.WithCancel(context.Background())
	e.Start(ctx)
	time.Sleep(60 * time.Millisecond)
	e.Stop()
	// Second Stop should be a no-op.
	e.Stop()
	cancel()
}

func TestRecordingEngine_Format(t *testing.T) {
	v := RecordingStateView{Name: "x", NewMetric: "x_v", Interval: time.Second, LastValue: 99, Runs: 5, Fails: 1}
	f := v.Format()
	if f["type"] != "recording" {
		t.Fatalf("type: %v", f["type"])
	}
	if f["value"] != float64(99) {
		t.Fatalf("value: %v", f["value"])
	}
}

func TestErrString(t *testing.T) {
	if errString(nil) != "" {
		t.Fatal("nil error should return empty string")
	}
	if errString(errors.New("x")) != "x" {
		t.Fatal("expected x")
	}
}
