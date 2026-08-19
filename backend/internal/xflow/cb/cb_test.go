package cb

import "testing"

func TestRegister(t *testing.T) {
	r := New[int]()
	id := r.Register(func(_ int) {})
	if id == 0 {
		t.Fatal("id")
	}
}

func TestTrigger(t *testing.T) {
	r := New[int]()
	called := 0
	r.Register(func(v int) {
		if v == 42 {
			called++
		}
	})
	r.Trigger(42)
	if called != 1 {
		t.Fatal("call", called)
	}
}

func TestUnregister(t *testing.T) {
	r := New[int]()
	id := r.Register(func(_ int) {})
	r.Unregister(id)
	if r.Count() != 0 {
		t.Fatal("unreg")
	}
}

func TestClear(t *testing.T) {
	r := New[int]()
	r.Register(func(_ int) {})
	r.Clear()
	if r.Count() != 0 {
		t.Fatal("clear")
	}
}
