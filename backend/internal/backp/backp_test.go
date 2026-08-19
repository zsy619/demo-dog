package backp

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestPushPop(t *testing.T) {
	c := New(4, 4, 1)
	defer c.Close()
	if err := c.Push([]byte("a")); err != nil {
		t.Fatal(err)
	}
	v, err := c.Pop()
	if err != nil || string(v) != "a" {
		t.Fatal("pop")
	}
}

func TestBackpressure(t *testing.T) {
	c := New(8, 4, 2)
	defer c.Close()
	// 填到 high
	for i := 0; i < 4; i++ {
		if err := c.Push([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	// 启动消费者
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 4; i++ {
			c.Pop()
		}
	}()
	// Push 应能继续（Pop 让水位降到 low 后）
	done := make(chan bool)
	go func() {
		c.Push([]byte("y"))
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("被卡住")
	}
	wg.Wait()
}

func TestTryPush(t *testing.T) {
	c := New(2, 2, 0)
	defer c.Close()
	if !c.TryPush([]byte("a")) {
		t.Fatal("应成功")
	}
	if !c.TryPush([]byte("b")) {
		t.Fatal("应成功")
	}
	if c.TryPush([]byte("c")) {
		t.Fatal("应失败")
	}
}

func TestClose(t *testing.T) {
	c := New(2, 2, 0)
	c.Close()
	if err := c.Push([]byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatal("应 ErrClosed")
	}
}

func TestLen(t *testing.T) {
	c := New(4, 4, 1)
	if c.Len() != 0 {
		t.Fatal("空")
	}
	c.Push([]byte("a"))
	if c.Len() != 1 {
		t.Fatal("len")
	}
}

func TestStats(t *testing.T) {
	c := New(4, 4, 1)
	c.Push([]byte("a"))
	c.Pop()
	s := c.Stats()
	if s.Written != 1 || s.Read != 1 {
		t.Fatal("stats")
	}
}
