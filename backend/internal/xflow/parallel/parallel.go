// Package parallel 提供并行执行任务并收集错误的辅助。
package parallel

import (
	"sync"
)

// Job 是单个并行任务。
type Job func() error

// All 并行执行所有任务，返回第一个错误（如有）。
func All(jobs ...Job) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(jobs))
	for _, j := range jobs {
		wg.Add(1)
		go func(job Job) {
			defer wg.Done()
			if err := job(); err != nil {
				errCh <- err
			}
		}(j)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// AllCollect 并行执行，收集所有错误。
func AllCollect(jobs ...Job) []error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(jobs))
	for _, j := range jobs {
		wg.Add(1)
		go func(job Job) {
			defer wg.Done()
			if err := job(); err != nil {
				errCh <- err
			}
		}(j)
	}
	wg.Wait()
	close(errCh)
	out := make([]error, 0)
	for err := range errCh {
		out = append(out, err)
	}
	return out
}

// Map 并行将 fn 应用于 items，并收集结果。
func Map[T any, U any](items []T, fn func(T) (U, error)) ([]U, []error) {
	out := make([]U, len(items))
	errs := make([]error, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(idx int, v T) {
			defer wg.Done()
			res, e := fn(v)
			out[idx] = res
			errs[idx] = e
		}(i, item)
	}
	wg.Wait()
	nonNil := make([]error, 0)
	for _, e := range errs {
		if e != nil {
			nonNil = append(nonNil, e)
		}
	}
	return out, nonNil
}
