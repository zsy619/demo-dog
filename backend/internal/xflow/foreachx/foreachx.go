// Package foreachx 提供并行遍历辅助。
package foreachx

import "sync"

// ForEach 并行执行 fn 处理 items，maxN 为最大并行度（<=0 不限）。
func ForEach[T any](items []T, maxN int, fn func(idx int, item T)) {
	if len(items) == 0 {
		return
	}
	if maxN <= 0 || maxN > len(items) {
		maxN = len(items)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < maxN; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				fn(i, items[i])
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

// Filter 返回满足 predicate 的元素。
func Filter[T any](items []T, predicate func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, v := range items {
		if predicate(v) {
			out = append(out, v)
		}
	}
	return out
}

// Reduce 折叠。
func Reduce[T, U any](items []T, init U, fn func(acc U, item T) U) U {
	acc := init
	for _, v := range items {
		acc = fn(acc, v)
	}
	return acc
}
