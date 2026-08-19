package circuit

import (
	"errors"
	"testing"
	"time"
)

func TestBreaker_Defaults(t *testing.T) {
	b := New(Settings{})
	if b.settings.FailureThreshold != 5 {
		t.Fatal("default threshold")
	}
	if b.settings.CoolDown != 30*time.Second {
		t.Fatal("default cool down")
	}
	if b.State() != StateClosed {
		t.Fatal("fresh breaker should be closed")
	}
	if err := b.Allow(); err != nil {
		t.Fatal("closed breaker allows")
	}
}

func TestBreaker_Opens_AfterThreshold(t *testing.T) {
	b := New(Settings{FailureThreshold: 3})
	for i := 0; i < 3; i++ {
		b.Failure()
	}
	if b.State() != StateOpen {
		t.Fatalf("expected open, got %s", b.State())
	}
	if err := b.Allow(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
}

func TestBreaker_Opens_BelowThreshold(t *testing.T) {
	b := New(Settings{FailureThreshold: 5})
	for i := 0; i < 4; i++ {
		b.Failure()
	}
	if b.State() == StateOpen {
		t.Fatal("should not open before threshold")
	}
	if err := b.Allow(); err != nil {
		t.Fatalf("should still allow: %v", err)
	}
}

func TestBreaker_SuccessResetsFailureCount(t *testing.T) {
	b := New(Settings{FailureThreshold: 3})
	b.Failure()
	b.Failure()
	b.Success()
	b.Failure()
	b.Failure()
	if b.State() == StateOpen {
		t.Fatal("two failures after success should not open")
	}
}

func TestBreaker_HalfOpen_AfterCoolDown(t *testing.T) {
	fakeNow := time.Unix(1700000000, 0)
	b := New(Settings{FailureThreshold: 1, CoolDown: time.Minute, Now: func() time.Time { return fakeNow }})
	b.Failure()
	if b.State() != StateOpen {
		t.Fatal("should be open")
	}
	fakeNow = fakeNow.Add(time.Minute + time.Second)
	if b.State() != StateHalfOpen {
		t.Fatalf("expected half_open, got %s", b.State())
	}
}

func TestBreaker_HalfOpen_OneTrialAllowed(t *testing.T) {
	fakeNow := time.Unix(1700000000, 0)
	b := New(Settings{FailureThreshold: 1, CoolDown: time.Minute, Now: func() time.Time { return fakeNow }})
	b.Failure()
	fakeNow = fakeNow.Add(time.Minute + time.Second)
	if err := b.Allow(); err != nil {
		t.Fatal("first half-open call should be allowed")
	}
	if err := b.Allow(); !errors.Is(err, ErrOpen) {
		t.Fatalf("second half-open call should be blocked: %v", err)
	}
}

func TestBreaker_HalfOpenSuccessCloses(t *testing.T) {
	fakeNow := time.Unix(1700000000, 0)
	b := New(Settings{FailureThreshold: 1, CoolDown: time.Minute, Now: func() time.Time { return fakeNow }})
	b.Failure()
	fakeNow = fakeNow.Add(time.Minute + time.Second)
	if err := b.Allow(); err != nil {
		t.Fatal("trial allowed")
	}
	b.Success()
	if b.State() != StateClosed {
		t.Fatalf("expected closed, got %s", b.State())
	}
	if err := b.Allow(); err != nil {
		t.Fatal("after success should allow")
	}
}

func TestBreaker_HalfOpenFailureReopens(t *testing.T) {
	fakeNow := time.Unix(1700000000, 0)
	b := New(Settings{FailureThreshold: 1, CoolDown: time.Minute, Now: func() time.Time { return fakeNow }})
	b.Failure()
	fakeNow = fakeNow.Add(time.Minute + time.Second)
	b.Allow()
	b.Failure()
	if b.State() != StateOpen {
		t.Fatalf("expected open after trial failed, got %s", b.State())
	}
}

func TestBreaker_StateString(t *testing.T) {
	cases := map[State]string{
		StateClosed:   "closed",
		StateOpen:     "open",
		StateHalfOpen: "half_open",
	}
	for s, want := range cases {
		if s.String() != want {
			t.Errorf("%d: %s", s, s.String())
		}
	}
	if State(99).String() != "unknown" {
		t.Fatal("unknown")
	}
}

func TestBreaker_Snapshot(t *testing.T) {
	b := New(Settings{FailureThreshold: 2, CoolDown: time.Minute})
	b.Failure()
	s := b.Snapshot()
	if s.State != "closed" {
		t.Fatalf("state: %s", s.State)
	}
	if s.Failures != 1 {
		t.Fatalf("failures: %d", s.Failures)
	}
	if s.OpenedAt != "" {
		t.Fatal("should not have opened_at yet")
	}
	b.Failure()
	s = b.Snapshot()
	if s.State != "open" {
		t.Fatalf("state: %s", s.State)
	}
	if s.OpenedAt == "" {
		t.Fatal("opened_at should be set")
	}
}

func TestBreaker_ConcurrentSafe(t *testing.T) {
	b := New(Settings{FailureThreshold: 100})
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				b.Failure()
				b.Success()
				_ = b.Allow()
				_ = b.Snapshot()
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
