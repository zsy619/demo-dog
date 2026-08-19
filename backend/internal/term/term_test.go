package term

import (
	"errors"
	"testing"
	"time"
)

func newClock() *Clock {
	return New("self", []string{"self", "peer1", "peer2"}, time.Second).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestHeartbeat_NewLeader(t *testing.T) {
	c := newClock()
	if err := c.Heartbeat("peer1", 1); err != nil {
		t.Fatal(err)
	}
	s := c.Snapshot()
	if s.Term != 1 || s.Leader != "peer1" || s.IsLeader {
		t.Fatalf("snapshot: %+v", s)
	}
}

func TestHeartbeat_StaleRejected(t *testing.T) {
	c := newClock()
	c.Heartbeat("peer1", 5)
	err := c.Heartbeat("peer2", 3)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale, got %v", err)
	}
	s := c.Snapshot()
	if s.Term != 5 || s.Leader != "peer1" {
		t.Fatalf("term: %d leader: %s", s.Term, s.Leader)
	}
}

func TestHeartbeat_NewerDemotes(t *testing.T) {
	c := newClock()
	c.MaybeElect()
	if !c.Snapshot().IsLeader {
		t.Fatal("should be leader")
	}
	if err := c.Heartbeat("peer1", 5); err != nil {
		t.Fatal(err)
	}
	if c.Snapshot().IsLeader {
		t.Fatal("should demote")
	}
}

func TestMaybeElect_NoLeader(t *testing.T) {
	c := newClock()
	if !c.MaybeElect() {
		t.Fatal("should elect")
	}
	s := c.Snapshot()
	if s.Term != 1 || !s.IsLeader || s.Leader != "self" {
		t.Fatalf("snapshot: %+v", s)
	}
}

func TestMaybeElect_AlreadyLeader(t *testing.T) {
	c := newClock()
	c.MaybeElect()
	if c.MaybeElect() {
		t.Fatal("should not re-elect")
	}
}

func TestMaybeElect_AfterHeartbeatTTL(t *testing.T) {
	now := time.Unix(1700000000, 0)
	c := New("self", []string{"self", "p"}, time.Second).WithTime(func() time.Time { return now })
	c.Heartbeat("p", 1)
	now = now.Add(2 * time.Second)
	if !c.MaybeElect() {
		t.Fatal("should elect after TTL")
	}
}

func TestMaybeElect_NoTTLNeeded(t *testing.T) {
	now := time.Unix(1700000000, 0)
	c := New("self", []string{"self", "p"}, time.Second).WithTime(func() time.Time { return now })
	c.Heartbeat("p", 1)
	now = now.Add(100 * time.Millisecond)
	if c.MaybeElect() {
		t.Fatal("should not elect within TTL")
	}
}

func TestStepDown(t *testing.T) {
	c := newClock()
	c.MaybeElect()
	c.StepDown()
	if c.Snapshot().IsLeader {
		t.Fatal("should not be leader")
	}
	if c.Stats().Stepdowns < 1 {
		t.Fatal("counter")
	}
}

func TestStats(t *testing.T) {
	c := newClock()
	if !c.MaybeElect() {
		t.Fatal("should elect (no leader yet)")
	}
	s := c.Stats()
	if s.Elections == 0 || s.Term < 1 {
		t.Fatalf("stats: %+v", s)
	}
}
