package mustx

import (
	"errors"
	"testing"
)

func TestNoError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("应 panic")
		}
	}()
	NoError(errors.New("boom"))
}

func TestNoErrorOK(t *testing.T) {
	NoError(nil)
}

func TestNoErrorFn(t *testing.T) {
	v := NoErrorFn(func() (int, error) { return 42, nil })
	if v != 42 {
		t.Fatal("fn", v)
	}
}

func TestTrue(t *testing.T) {
	True(1 == 1, "ok")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("应 panic")
		}
	}()
	True(1 == 2, "no")
}

func TestNotNil(t *testing.T) {
	NotNil(1, "ok")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("应 panic")
		}
	}()
	NotNil(nil, "nil")
}
