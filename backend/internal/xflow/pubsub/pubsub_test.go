package pubsub

import (
	"testing"
	"time"
)

func TestPublishReceive(t *testing.T) {
	b := NewBus()
	s := b.Subscribe("a", 8)
	defer s.Close()
	b.Publish("a", []byte("x"))
	select {
	case m := <-s.Messages():
		if string(m.Payload) != "x" {
			t.Fatal("payload")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestPublish_PerSub(t *testing.T) {
	b := NewBus()
	s1 := b.Subscribe("a", 8)
	s2 := b.Subscribe("a", 8)
	defer s1.Close()
	defer s2.Close()
	b.Publish("a", []byte("y"))
	for _, s := range []*Subscriber{s1, s2} {
		select {
		case m := <-s.Messages():
			if string(m.Payload) != "y" {
				t.Fatal("payload")
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}
}

func TestPublish_Bounded(t *testing.T) {
	b := NewBus()
	s := b.Subscribe("a", 1)
	defer s.Close()
	for i := 0; i < 5; i++ {
		b.Publish("a", []byte("x"))
	}
	if s.Dropped() == 0 {
		t.Fatal("should drop some")
	}
}

func TestClose_Subscriber(t *testing.T) {
	b := NewBus()
	s := b.Subscribe("a", 8)
	s.Close()
	_, ok := <-s.Messages()
	if ok {
		t.Fatal("should be closed")
	}
}

func TestClose_Topic(t *testing.T) {
	b := NewBus()
	s := b.Subscribe("a", 8)
	b.CloseTopic("a")
	_, ok := <-s.Messages()
	if ok {
		t.Fatal("should be closed")
	}
}

func TestPublish_ClosedSub(t *testing.T) {
	b := NewBus()
	s := b.Subscribe("a", 8)
	s.Close()
	b.Publish("a", []byte("x"))
	// Should not panic, should not block.
}

func TestStats(t *testing.T) {
	b := NewBus()
	s := b.Subscribe("a", 8)
	defer s.Close()
	b.Publish("a", []byte("x"))
	st := b.Stats()
	if st.Topics != 1 || st.Subscribers != 1 || st.Published != 1 {
		t.Fatal("stats")
	}
}

func TestPublish_NoSubs(t *testing.T) {
	b := NewBus()
	b.Publish("missing", []byte("x"))
	// Should not panic.
}
