package lrumap

import (
	"sync"
	"testing"
)

func TestConcurrent(t *testing.T) {
	m := New[string, int](64)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			m.Put(string(rune(97+i%26)), i)
		}(i)
		go func(i int) {
			defer wg.Done()
			m.Get(string(rune(97+i%26)))
		}(i)
	}
	wg.Wait()
}
