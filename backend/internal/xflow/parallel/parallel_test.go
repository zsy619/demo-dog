package parallel

import (
	"context"
	"errors"
	"testing"
)

func TestAllOk(t *testing.T) {
	err := All(
		func() error { return nil },
		func() error { return nil },
	)
	if err != nil {
		t.Fatal("err", err)
	}
}

func TestAllErr(t *testing.T) {
	myErr := errors.New("x")
	err := All(
		func() error { return nil },
		func() error { return myErr },
		func() error { return nil },
	)
	if err == nil {
		t.Fatal("应报错")
	}
}

func TestAllCollect(t *testing.T) {
	e1 := errors.New("1")
	e2 := errors.New("2")
	errs := AllCollect(
		func() error { return e1 },
		func() error { return nil },
		func() error { return e2 },
	)
	if len(errs) != 2 {
		t.Fatal("collect", len(errs))
	}
}

func TestMap(t *testing.T) {
	out, errs := Map([]int{1, 2, 3}, func(v int) (int, error) { return v * 10, nil })
	if len(errs) != 0 {
		t.Fatal("errs")
	}
	if out[0] != 10 || out[2] != 30 {
		t.Fatal("map", out)
	}
}

func TestPanic(t *testing.T) {
	err := All(func() error { panic("boom") })
	if err == nil {
		t.Fatal("panic 应转为 error")
	}
}

func TestAllCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := AllCtx(ctx, func(_ context.Context) error { return nil })
	// 已 cancel 但 fn 立即成功，无 error 也合理
	if err != nil {
		t.Fatal("应 nil")
	}
}

func TestForEachCtx(t *testing.T) {
	errs := ForEachCtx(context.Background(), []int{1, 2, 3}, 2, func(_ context.Context, idx int, v int) error {
		if v == 2 {
			return errors.New("err-2")
		}
		return nil
	})
	if len(errs) != 1 {
		t.Fatal("应 1 个 err", errs)
	}
}

func TestMapEmpty(t *testing.T) {
	out, errs := Map([]int{}, func(v int) (int, error) { return v, nil })
	if len(out) != 0 || len(errs) != 0 {
		t.Fatal("空切片应返回 nil")
	}
}

func TestAllEmpty(t *testing.T) {
	if err := All(); err != nil {
		t.Fatal("空应返回 nil")
	}
}
