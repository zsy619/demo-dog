// Package mergex 提供一个简单的合并器：
// 多个 chan 合并为一个输出 chan。
package mergex

import "sync"

// Merge 把多个只读 chan 合并为一个。
// 当所有 src 都关闭时 out 自动关闭。
func Merge[T any](out chan<- T, srcs ...<-chan T) {
	var wg sync.WaitGroup
	for _, src := range srcs {
		wg.Add(1)
		go func(c <-chan T) {
			defer wg.Done()
			for v := range c {
				out <- v
			}
		}(src)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
}

// Fanout 把一个 chan 扇出到 n 个 chan。
func Fanout[T any](in <-chan T, n int) []<-chan T {
	outs := make([]chan T, n)
	for i := 0; i < n; i++ {
		outs[i] = make(chan T)
	}
	go func() {
		defer func() {
			for _, c := range outs {
				close(c)
			}
		}()
		for v := range in {
			for _, c := range outs {
				c <- v
			}
		}
	}()
	res := make([]<-chan T, n)
	for i, c := range outs {
		res[i] = c
	}
	return res
}
