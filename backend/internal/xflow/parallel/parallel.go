// Package parallel 提供并行执行辅助：
// 任务执行、错误收集、迭代并行。
//
// 所有函数：
//   - fn 中 panic 被恢复并转为 error（不会杀死整个调用）
//   - 支持 ctx 取消（Context 变体）
//   - 线程安全地收集错误到返回切片
package parallel

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Job 是单个并行任务。
type Job func() error

// CtxJob 是接收 ctx 的并行任务。
type CtxJob func(ctx context.Context) error

// All 并行执行所有任务，返回第一个非 nil 错误。
// 若无错误返回 nil。
func All(jobs ...Job) error {
	if len(jobs) == 0 {
		return nil
	}
	errs := runJobs(jobs)
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// AllCollect 并行执行，收集所有非 nil 错误。
func AllCollect(jobs ...Job) []error {
	if len(jobs) == 0 {
		return nil
	}
	return runJobs(jobs)
}

// AllCtx 并行执行带 ctx 的任务；首个错误返回并取消 ctx。
func AllCtx(ctx context.Context, jobs ...CtxJob) error {
	if len(jobs) == 0 {
		return ctx.Err()
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make([]error, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(idx int, job CtxJob) {
			defer wg.Done()
			out[idx] = safeRunCtx(runCtx, job)
		}(i, j)
	}
	wg.Wait()
	for _, e := range out {
		if e != nil {
			return e
		}
	}
	return nil
}

// Map 并行将 fn 应用于 items，结果与错误按原顺序返回。
func Map[T any, U any](items []T, fn func(T) (U, error)) ([]U, []error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]U, len(items))
	errs := make([]error, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(idx int, v T) {
			defer wg.Done()
			res, e := safeRunValue(v, fn)
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

// MapCtx 并行 map（带 ctx）。
func MapCtx[T any, U any](ctx context.Context, items []T, fn func(ctx context.Context, v T) (U, error)) ([]U, []error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]U, len(items))
	errs := make([]error, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(idx int, v T) {
			defer wg.Done()
			res, e := safeRunValueCtx(ctx, v, fn)
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

// ForEach 并行处理 items，maxN 限制最大并发（<=0 或 > len 视为不限）。
func ForEach[T any](items []T, maxN int, fn func(idx int, item T)) {
	forEachCtx(items, maxN, nil, func(_ context.Context, idx int, v T) { fn(idx, v) })
}

// ForEachCtx 并行处理（带 ctx 与错误收集）。
func ForEachCtx[T any](ctx context.Context, items []T, maxN int, fn func(ctx context.Context, idx int, item T) error) []error {
	if len(items) == 0 {
		return nil
	}
	if maxN <= 0 || maxN > len(items) {
		maxN = len(items)
	}
	jobs := make(chan int, len(items))
	errs := make([]error, len(items))
	var wg sync.WaitGroup
	for w := 0; w < maxN; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					errs[i] = ctx.Err()
					continue
				}
				if e := safeRunCtxGeneric(items, i, ctx, fn); e != nil {
					errs[i] = e
				}
			}
		}()
	}
	for i := range items {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return compactErrs(errs)
		}
	}
	close(jobs)
	wg.Wait()
	return compactErrs(errs)
}

// 兼容旧的无 ctx fn
func forEachCtx[T any](items []T, maxN int, _ context.Context, fn func(ctx context.Context, idx int, item T)) {
	if len(items) == 0 {
		return
	}
	if maxN <= 0 || maxN > len(items) {
		maxN = len(items)
	}
	jobs := make(chan int, len(items))
	var wg sync.WaitGroup
	for w := 0; w < maxN; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				fn(context.Background(), i, items[i])
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

// 内部 helpers

func runJobs(jobs []Job) []error {
	out := make([]error, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(idx int, job Job) {
			defer wg.Done()
			out[idx] = safeRun(job)
		}(i, j)
	}
	wg.Wait()
	return compactErrs(out)
}

func safeRun(job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parallel: panic: %v", r)
		}
	}()
	return job()
}

func safeRunCtx(ctx context.Context, job CtxJob) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parallel: panic: %v", r)
		}
	}()
	return job(ctx)
}

func safeRunCtxGeneric[T any](items []T, i int, ctx context.Context, fn func(ctx context.Context, idx int, item T) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parallel: panic: %v", r)
		}
	}()
	return fn(ctx, i, items[i])
}

func safeRunValue[T any, U any](v T, fn func(T) (U, error)) (res U, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parallel: panic: %v", r)
		}
	}()
	return fn(v)
}

func safeRunValueCtx[T any, U any](ctx context.Context, v T, fn func(ctx context.Context, v T) (U, error)) (res U, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parallel: panic: %v", r)
		}
	}()
	return fn(ctx, v)
}

func compactErrs(errs []error) []error {
	out := make([]error, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// IsPanicErr 判断 err 是否由 panic 转换而来。
func IsPanicErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, err) && contains(err.Error(), "panic")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
