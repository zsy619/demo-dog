package orderedmap

import "testing"

func TestSetGet(t *testing.T) {
	o := New()
	o.Set("a", 1)
	v, ok := o.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestOrder(t *testing.T) {
	o := New()
	o.Set("b", 2)
	o.Set("a", 1)
	o.Set("c", 3)
	if got := o.Keys(); got[0] != "b" || got[1] != "a" || got[2] != "c" {
		t.Fatal("order")
	}
}

func TestUpdate(t *testing.T) {
	o := New()
	o.Set("a", 1)
	o.Set("a", 2)
	if len(o.Keys()) != 1 {
		t.Fatal("update 应保留顺序")
	}
	v, _ := o.Get("a")
	if v.(int) != 2 {
		t.Fatal("val")
	}
}

func TestDelete(t *testing.T) {
	o := New()
	o.Set("a", 1)
	o.Set("b", 2)
	o.Delete("a")
	keys := o.Keys()
	if len(keys) != 1 || keys[0] != "b" {
		t.Fatal("del")
	}
}

func TestRange(t *testing.T) {
	o := New()
	o.Set("a", 1)
	o.Set("b", 2)
	var sum int
	o.Range(func(_ string, v any) bool { sum += v.(int); return true })
	if sum != 3 {
		t.Fatal("range")
	}
}

func TestRange_Stop(t *testing.T) {
	o := New()
	o.Set("a", 1)
	o.Set("b", 2)
	count := 0
	o.Range(func(_ string, _ any) bool { count++; return false })
	if count != 1 {
		t.Fatal("stop")
	}
}

func TestClear(t *testing.T) {
	o := New()
	o.Set("a", 1)
	o.Clear()
	if o.Len() != 0 {
		t.Fatal("clear")
	}
}
