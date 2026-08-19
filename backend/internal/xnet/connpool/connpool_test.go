package connpool

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newPool() *Pool {
	return New(2, timeMin(), func(_ context.Context) (func(), error) {
		return func() {}, nil
	})
}

func timeMin() time.Duration { return time.Minute }

func TestGetPut(t *testing.T) {
	p := newPool()
	c, err := p.Get(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	p.Put(c)
	c2, _ := p.Get(context.Background(), "k")
	if c2.id != c.id {
		t.Fatal("应复用")
	}
}

func TestGet_DialError(t *testing.T) {
	p := New(1, time.Minute, func(_ context.Context) (func(), error) {
		return nil, errors.New("dial fail")
	})
	if _, err := p.Get(context.Background(), "x"); err == nil {
		t.Fatal("应报错")
	}
}

func TestCapacity(t *testing.T) {
	p := newPool()
	c1, _ := p.Get(context.Background(), "k")
	c2, _ := p.Get(context.Background(), "k")
	p.Put(c1)
	p.Put(c2)
	c3, _ := p.Get(context.Background(), "k")
	p.Put(c3) // 此时 idle = [c3, c2] len 2 == cap; 应关闭
	p.Put(c3) // 第二次同上
	s := p.Stats()
	if s.Idle > 2 {
		t.Fatal("应不超过容量")
	}
}

func TestClose(t *testing.T) {
	p := newPool()
	c, _ := p.Get(context.Background(), "k")
	p.Put(c)
	p.Close()
	if p.Stats().Idle != 0 {
		t.Fatal("close 后应清空")
	}
}

func TestStats(t *testing.T) {
	p := newPool()
	c1, _ := p.Get(context.Background(), "a")
	c2, _ := p.Get(context.Background(), "b")
	p.Put(c1)
	p.Put(c2)
	s := p.Stats()
	if s.ByKey["a"] != 1 || s.ByKey["b"] != 1 {
		t.Fatal("bykey")
	}
}
