package warm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memSink struct {
	mu sync.Mutex
	m  map[string]any
}

func newMemSink() *memSink { return &memSink{m: map[string]any{}} }

func (s *memSink) Put(_ context.Context, k string, v any) error {
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
	return nil
}

func TestWarm_AllSuccess(t *testing.T) {
	sink := newMemSink()
	ld := func(_ context.Context, k string) (any, error) {
		return "v-" + k, nil
	}
	jobs := []Job{{Name: "a", Keys: []string{"x", "y"}}}
	st, err := Warm(context.Background(), Default(), ld, sink, jobs)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 2 || st.Loaded != 2 || st.Failed != 0 {
		t.Fatal("stats")
	}
	if sink.m["x"].(string) != "v-x" {
		t.Fatal("val")
	}
}

func TestWarm_PartialFail(t *testing.T) {
	sink := newMemSink()
	ld := func(_ context.Context, k string) (any, error) {
		if k == "bad" {
			return nil, errors.New("boom")
		}
		return "ok", nil
	}
	jobs := []Job{{Keys: []string{"good", "bad"}}}
	st, _ := Warm(context.Background(), Config{Concurrency: 2, Timeout: 100 * time.Millisecond, Retry: 0}, ld, sink, jobs)
	if st.Loaded != 1 || st.Failed != 1 {
		t.Fatal("partial")
	}
}

func TestWarm_AllFail(t *testing.T) {
	sink := newMemSink()
	ld := func(_ context.Context, k string) (any, error) { return nil, errors.New("x") }
	jobs := []Job{{Keys: []string{"a"}}}
	_, err := Warm(context.Background(), Config{Concurrency: 1, Timeout: 100 * time.Millisecond, Retry: 0}, ld, sink, jobs)
	if err == nil {
		t.Fatal("全部失败应报错")
	}
}

func TestWarm_Retry(t *testing.T) {
	sink := newMemSink()
	calls := 0
	ld := func(_ context.Context, k string) (any, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("first")
		}
		return "ok", nil
	}
	jobs := []Job{{Keys: []string{"k"}}}
	st, _ := Warm(context.Background(), Config{Concurrency: 1, Timeout: 100 * time.Millisecond, Retry: 2}, ld, sink, jobs)
	if st.Loaded != 1 {
		t.Fatal("retry")
	}
}
