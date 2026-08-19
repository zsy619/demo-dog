package triggerx

import "testing"

func TestFire(t *testing.T) {
	tr := New[int]()
	var sum int
	tr.Add(func(v int) { sum += v })
	tr.Add(func(v int) { sum += v * 10 })
	tr.Fire(3)
	if sum != 33 {
		t.Fatal("fire", sum)
	}
}

func TestCancel(t *testing.T) {
	tr := New[int]()
	var sum int
	cancel := tr.Add(func(v int) { sum += v })
	cancel()
	tr.Fire(5)
	if sum != 0 {
		t.Fatal("cancel", sum)
	}
}

func TestCount(t *testing.T) {
	tr := New[int]()
	tr.Add(func(int) {})
	tr.Add(func(int) {})
	if tr.Count() != 2 {
		t.Fatal("count")
	}
}

func TestClear(t *testing.T) {
	tr := New[int]()
	tr.Add(func(int) {})
	tr.Clear()
	if tr.Count() != 0 {
		t.Fatal("clear")
	}
}
