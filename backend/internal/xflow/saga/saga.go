// Package saga 分布式事务 saga：步骤编排 + 失败补偿。
package saga

import (
	"context"
	"errors"
	"sync"
)

// Step is one unit of work in a saga.
type Step struct {
	Name   string
	Do     func(ctx context.Context) error
	Undo   func(ctx context.Context) error
	Result any
}

// Outcome is the recorded outcome of a saga.
type Outcome struct {
	Name     string
	Executed []string
	Error    error
	Undone   []string
	Rollback bool
}

// Coordinator runs the steps in order; on the first error
// it runs each previously executed step's Undo in reverse
// order to compensate.
type Coordinator struct {
	mu       sync.Mutex
	onRollback func(o Outcome)
}

// New returns an empty coordinator.
func New() *Coordinator {
	return &Coordinator{}
}

// OnRollback registers a hook fired after rollback completes.
func (c *Coordinator) OnRollback(fn func(o Outcome)) {
	c.mu.Lock()
	c.onRollback = fn
	c.mu.Unlock()
}

// Run executes the saga. If a step fails the previous steps
// are compensated and the outcome records Rollback=true.
func (c *Coordinator) Run(ctx context.Context, steps []Step) Outcome {
	out := Outcome{Executed: make([]string, 0, len(steps))}
	for i, s := range steps {
		if err := ctx.Err(); err != nil {
			out.Error = err
			out.Name = s.Name
			c.compensate(ctx, steps[:i], &out)
			return out
		}
		err := s.Do(ctx)
		out.Executed = append(out.Executed, s.Name)
		if err != nil {
			out.Error = err
			out.Name = s.Name
			c.compensate(ctx, steps[:i], &out)
			return out
		}
		if s.Result != nil {
			out.Executed = append(out.Executed, "+result")
		}
	}
	return out
}

func (c *Coordinator) compensate(ctx context.Context, steps []Step, out *Outcome) {
	out.Rollback = true
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Undo == nil {
			continue
		}
		_ = steps[i].Undo(ctx)
		out.Undone = append(out.Undone, steps[i].Name)
	}
	c.mu.Lock()
	hook := c.onRollback
	c.mu.Unlock()
	if hook != nil {
		hook(*out)
	}
}

// ErrEmpty is returned when Run is given zero steps.
var ErrEmpty = errors.New("saga has no steps")
