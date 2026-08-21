// Package pipeline Pipeline 模式：阶段链式处理，支持 ctx 取消。
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Stage 处理一个输入并产生一个（可能
// transformed) output or an error. The pipeline feeds the
// output of stage N into stage N+1.
type Stage[T any] func(ctx context.Context, in T) (T, error)

// Pipeline 将多个 stage 链接起来。
type Pipeline[T any] struct {
	stages []Stage[T]
	name   string
}

// New 由 stages 创建一个具名流水线。
func New[T any](name string, stages ...Stage[T]) *Pipeline[T] {
	return &Pipeline[T]{name: name, stages: stages}
}

// Run 按顺序执行阶段。
func (p *Pipeline[T]) Run(ctx context.Context, in T) (T, error) {
	var zero T
	if len(p.stages) == 0 {
		return zero, ErrEmpty
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	v := in
	for i, s := range p.stages {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, fmt.Errorf("pipeline %q stage %d: %w", p.name, i, err)
		}
		out, err := s(ctx, v)
		if err != nil {
			var zero T
			return zero, fmt.Errorf("pipeline %q stage %d: %w", p.name, i, err)
		}
		v = out
	}
	return v, nil
}

// ForkResult 是单个分支的输出。
type ForkResult[T any] struct {
	Name   string
	Value  T
	Err    error
}

// Fork 并发执行每个阶段，使用相同输入
// and 返回 each branch result. Context-aware: if ctx is
// cancelled all branches see Done.
func Fork[T any](ctx context.Context, branches map[string]Stage[T], in T) []ForkResult[T] {
	results := make([]ForkResult[T], 0, len(branches))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var pending atomic.Int32
	pending.Add(int32(len(branches)))
	for name, s := range branches {
		name, s := name, s
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer pending.Add(-1)
			v, err := s(ctx, in)
			mu.Lock()
			results = append(results, ForkResult[T]{Name: name, Value: v, Err: err})
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}

// FirstError 从结果中返回第一个非 nil 错误。
func FirstError[T any](results []ForkResult[T]) error {
	for _, r := range results {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

// ErrEmpty 在流水线没有阶段时返回。
var ErrEmpty = errors.New("pipeline has no stages")
