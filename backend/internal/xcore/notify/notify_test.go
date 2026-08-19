package notify

import (
	"errors"
	"testing"
	"time"
)

func TestSubscribeReceive(t *testing.T) {
	b := New()
	defer b.Close()
	ch, unsub := b.Subscribe("a", 4)
	defer unsub()
	if err := b.Publish("a", "x"); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if ev.Payload.(string) != "x" {
			t.Fatal("payload")
		}
	case <-time.After(time.Second):
		t.Fatal("超时")
	}
}

func TestMultipleSubs(t *testing.T) {
	b := New()
	defer b.Close()
	ch1, u1 := b.Subscribe("a", 4)
	ch2, u2 := b.Subscribe("a", 4)
	defer u1()
	defer u2()
	b.Publish("a", 1)
	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("超时")
		}
	}
}

func TestPublish_Bounded(t *testing.T) {
	b := New()
	defer b.Close()
	ch, _ := b.Subscribe("a", 1)
	for i := 0; i < 5; i++ {
		b.Publish("a", i)
	}
	s := b.Stats()
	if s.Dropped == 0 {
		t.Fatal("应丢弃")
	}
	_ = ch
}

func TestUnsub(t *testing.T) {
	b := New()
	defer b.Close()
	ch, unsub := b.Subscribe("a", 1)
	unsub()
	_, ok := <-ch
	if ok {
		t.Fatal("应关闭")
	}
}

func TestPublish_Closed(t *testing.T) {
	b := New()
	b.Close()
	if err := b.Publish("a", 1); !errors.Is(err, ErrClosed) {
		t.Fatal("应 ErrClosed")
	}
}

func TestStats(t *testing.T) {
	b := New()
	defer b.Close()
	ch, unsub := b.Subscribe("a", 4)
	defer unsub()
	b.Publish("a", 1)
	<-ch
	s := b.Stats()
	if s.TotalIn != 1 || s.TotalOut != 1 {
		t.Fatal("stats")
	}
}
