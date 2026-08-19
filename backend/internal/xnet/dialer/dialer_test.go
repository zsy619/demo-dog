package dialer

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDial_NotCooling(t *testing.T) {
	d := New(Config{BaseTTL: 100 * time.Millisecond}, (&net.Dialer{}).DialContext)
	if d.isCooling("nope") {
		t.Fatal("应不冷却")
	}
}

func TestDial_CoolingAfterFail(t *testing.T) {
	failing := func(_ context.Context, _, addr string) (net.Conn, error) {
		return nil, errors.New("dial fail")
	}
	d := New(Config{BaseTTL: 100 * time.Millisecond, MaxTTL: 200 * time.Millisecond}, failing)
	_, err := d.Dial(context.Background(), "tcp", "127.0.0.1:1")
	if err == nil {
		t.Fatal("应失败")
	}
	if !d.isCooling("127.0.0.1") {
		t.Fatal("应被冷却")
	}
}

func TestDial_CoolingBlocks(t *testing.T) {
	failing := func(_ context.Context, _, addr string) (net.Conn, error) {
		return nil, errors.New("x")
	}
	d := New(Config{BaseTTL: 200 * time.Millisecond, MaxTTL: time.Second}, failing)
	d.Dial(context.Background(), "tcp", "127.0.0.1:1")
	_, err := d.Dial(context.Background(), "tcp", "127.0.0.1:1")
	if !errors.Is(err, ErrCooling) {
		t.Fatal("应 ErrCooling")
	}
}

func TestDial_ClearOnSuccess(t *testing.T) {
	calls := 0
	good := func(_ context.Context, _, _ string) (net.Conn, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("tmp")
		}
		return &net.TCPConn{}, nil
	}
	d := New(Config{BaseTTL: 200 * time.Millisecond, MaxTTL: 500 * time.Millisecond}, good)
	_, err1 := d.Dial(context.Background(), "tcp", "127.0.0.1:1")
	if err1 == nil {
		t.Fatal("第一次应失败")
	}
	if d.isCooling("127.0.0.1") == false {
		t.Fatal("应冷却")
	}
	// 手动清除冷却以模拟时间过去
	d.Reset()
	_, err2 := d.Dial(context.Background(), "tcp", "127.0.0.1:1")
	if err2 != nil {
		t.Fatal("第二次应成功")
	}
	if d.isCooling("127.0.0.1") {
		t.Fatal("成功后应清除")
	}
}

func TestReset(t *testing.T) {
	d := New(Config{BaseTTL: time.Second}, nil)
	d.recordFailure("a")
	d.Reset()
	if d.isCooling("a") {
		t.Fatal("reset")
	}
}

func TestCooldownUntil(t *testing.T) {
	d := New(Config{BaseTTL: time.Second}, nil)
	d.recordFailure("a")
	if _, ok := d.CooldownUntil("a"); !ok {
		t.Fatal("应有值")
	}
}
