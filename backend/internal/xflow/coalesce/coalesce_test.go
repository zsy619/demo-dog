package coalesce

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleCall(t *testing.T) {
	c := New[string, int]()
	v, err := c.Do("k", func() (int, error) { return 42, nil })
	if err != nil || v != 42 {
		t.Fatal("single", v, err)
	}
}

func TestCoalesce(t *testing.T) {
	var calls atomic.Int32
	c := New[string, int]()
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		<-start
		time.Sleep(20 * time.Millisecond)
		v, _ := c.Do("k", func() (int, error) { return 1, nil })
		if v != 100 {
			t.Fatal("coalesce", v)
		}
		close(done)
	}()
	close(start)
	v, _ := c.Do("k", func() (int, error) {
		calls.Add(1)
		time.Sleep(100 * time.Millisecond)
		return 100, nil
	})
	if v != 100 || calls.Load() != 1 {
		t.Fatal("first", v, calls.Load())
	}
	<-done
}

func TestErr(t *testing.T) {
	myErr := errors.New("x")
	c := New[string, int]()
	_, err := c.Do("k", func() (int, error) { return 0, myErr })
	if err != myErr {
		t.Fatal("err")
	}
}

func TestInflight(t *testing.T) {
	c := New[string, int]()
	if c.Inflight() != 0 {
		t.Fatal("0")
	}
}
