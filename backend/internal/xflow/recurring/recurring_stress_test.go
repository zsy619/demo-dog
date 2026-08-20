package recurring

import (
	"sync"
	"testing"
	"time"
)

func TestStartStopConcurrent(t *testing.T) {
	r := New(10*time.Millisecond, func(_ int64) {})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			r.Start()
		}()
		go func() {
			defer wg.Done()
			r.Stop()
		}()
	}
	wg.Wait()
}
