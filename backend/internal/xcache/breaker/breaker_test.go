package breaker

import (
	"errors"
	"testing"
	"time"
)

func newB() *Breaker {
	return New(Config{
		Window: time.Second, MinSamples: 3,
		FailureRatio: 0.5, OpenTimeout: time.Second,
	}).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestAllow_Closed(t *testing.T) {
	b := newB()
	if !b.Allow() {
		t.Fatal("should allow")
	}
}

func TestOpenAfterFailures(t *testing.T) {
	b := newB()
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Failure()
	}
	if b.State() != StateOpen {
		t.Fatal("should be open")
	}
	if b.Allow() {
		t.Fatal("should be denied")
	}
}

func TestHalfOpenAfterTimeout(t *testing.T) {
	now := time.Unix(1700000000, 0)
	b := New(Config{
		Window: time.Second, MinSamples: 3,
		FailureRatio: 0.5, OpenTimeout: time.Second,
	}).WithTime(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Failure()
	}
	if b.State() != StateOpen {
		t.Fatal("open")
	}
	now = now.Add(2 * time.Second)
	if b.State() != StateHalfOpen {
		t.Fatal("should be half-open")
	}
	if !b.Allow() {
		t.Fatal("half-open should allow")
	}
}

func TestHalfOpenSuccessCloses(t *testing.T) {
	now := time.Unix(1700000000, 0)
	b := New(Config{
		Window: time.Second, MinSamples: 3,
		FailureRatio: 0.5, OpenTimeout: time.Second,
	}).WithTime(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Failure()
	}
	now = now.Add(2 * time.Second)
	b.Allow()
	b.Success()
	if b.State() != StateClosed {
		t.Fatal("should close")
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	now := time.Unix(1700000000, 0)
	b := New(Config{
		Window: time.Second, MinSamples: 3,
		FailureRatio: 0.5, OpenTimeout: time.Second,
	}).WithTime(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Failure()
	}
	now = now.Add(2 * time.Second)
	b.Allow()
	b.Failure()
	if b.State() != StateOpen {
		t.Fatal("should reopen")
	}
}

func TestSuccessBelowThreshold(t *testing.T) {
	b := newB()
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Success()
	}
	if b.State() != StateClosed {
		t.Fatal("should stay closed")
	}
}

func TestStats(t *testing.T) {
	b := newB()
	b.Allow()
	b.Success()
	b.Allow()
	b.Failure()
	s := b.Stats()
	if s.Success != 1 || s.Failed != 1 {
		t.Fatalf("stats: %+v", s)
	}
}

func TestShortCircuit(t *testing.T) {
	b := newB()
	if err := b.ShortCircuit(); !errors.Is(err, ErrOpen) {
		t.Fatal(err)
	}
}

func TestStateString(t *testing.T) {
	if StateClosed.String() != "closed" {
		t.Fatal("closed")
	}
	if StateOpen.String() != "open" {
		t.Fatal("open")
	}
	if StateHalfOpen.String() != "half-open" {
		t.Fatal("half-open")
	}
}
