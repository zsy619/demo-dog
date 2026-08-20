package lrukv

import (
	"sync"
	"testing"
)

func TestConcurrent(t *testing.T) {
	k := New(64)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			k.Put(string(rune(97+i%26)), []byte{byte(i)})
		}(i)
		go func(i int) {
			defer wg.Done()
			k.Get(string(rune(97+i%26)))
		}(i)
	}
	wg.Wait()
}
