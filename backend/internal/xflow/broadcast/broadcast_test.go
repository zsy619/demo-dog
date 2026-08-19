package broadcast

import (
	"sync"
	"testing"
	"time"
)

func TestSubscribePublish(t *testing.T) {
	b := New[int]()
	ch, cancel := b.Subscribe(4)
	defer cancel()
	b.Publish(42)
	select {
	case v := <-ch:
		if v != 42 {
			t.Fatal("v")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestMultipleSubs(t *testing.T) {
	b := New[string]()
	a, ca := b.Subscribe(2)
	c2, cb := b.Subscribe(2)
	defer ca()
	defer cb()
	if n := b.Publish("hi"); n != 2 {
		t.Fatal("n", n)
	}
	if <-a != "hi" || <-c2 != "hi" {
		t.Fatal("recv")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := New[int]()
	_, cancel := b.Subscribe(1)
	cancel()
	if b.Subs() != 0 {
		t.Fatal("unsub")
	}
}

func TestDropSlowConsumer(t *testing.T) {
	b := New[int]()
	_, cancel := b.Subscribe(1)
	defer cancel()
	b.Publish(1)
	b.Publish(2)
}

func TestConcurrent(t *testing.T) {
	b := New[int]()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Publish(1)
		}()
	}
	wg.Wait()
}
