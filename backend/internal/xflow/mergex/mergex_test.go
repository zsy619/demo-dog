package mergex

import (
	"sync"
	"testing"
	"time"
)

func TestMerge(t *testing.T) {
	a := make(chan int, 2)
	b := make(chan int, 2)
	a <- 1
	b <- 10
	a <- 2
	b <- 20
	close(a)
	close(b)
	out := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	var got int
	go func() {
		defer wg.Done()
		for v := range out {
			got += v
		}
	}()
	Merge(out, a, b)
	wg.Wait()
	if got != 33 {
		t.Fatal("sum", got)
	}
}

func TestFanout(t *testing.T) {
	in := make(chan int, 2)
	in <- 1
	in <- 2
	close(in)
	chans := Fanout(in, 3)
	sum := 0
	for _, c := range chans {
		for v := range c {
			sum += v
		}
	}
	// 3 chans x 2 values x 1+2 = 9
	if sum != 9 {
		t.Fatal("fanout", sum)
	}
}

func TestMergeEmpty(t *testing.T) {
	out := make(chan int)
	done := make(chan struct{})
	go func() {
		for range out {
		}
		close(done)
	}()
	Merge[int](out)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
