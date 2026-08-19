package parallel

import (
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
