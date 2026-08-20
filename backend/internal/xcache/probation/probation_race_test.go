package probation

import (
	"sync"
	"testing"
)

func TestConcurrent(t *testing.T) {
	c := New(64)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.Put(string(rune(97+i%26)), i)
		}(i)
		go func(i int) {
			defer wg.Done()
			c.Get(string(rune(97+i%26)))
		}(i)
	}
	wg.Wait()
}
