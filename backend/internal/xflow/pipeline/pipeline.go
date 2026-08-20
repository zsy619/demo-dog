// Package pipeline Pipeline 模式：阶段链式处理，支持 ctx 取消。
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Stage processes one input and produces one (possibly
// transformed) output or an error. The pipeline feeds the
// output of stage N into stage N+1.
type Stage[T any] func(ctx context.Context, in T) (T, error)

// Pipeline chains stages together.
type Pipeline[T any] struct {
	stages []Stage[T]
	name   string
}

// New creates a named pipeline from stages.
func New[T any](name string, stages ...Stage[T]) *Pipeline[T] {
	return &Pipeline[T]{name: name, stages: stages}
}

// Run executes stages sequentially.
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

// ForkResult is one branch output.
type ForkResult[T any] struct {
	Name   string
	Value  T
	Err    error
}

// Fork executes each stage concurrently with the same input
// and returns each branch result. Context-aware: if ctx is
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

// FirstError returns the first non-nil error from results.
func FirstError[T any](results []ForkResult[T]) error {
	for _, r := range results {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

// ErrEmpty is returned when a pipeline has no stages.
var ErrEmpty = errors.New("pipeline has no stages")
