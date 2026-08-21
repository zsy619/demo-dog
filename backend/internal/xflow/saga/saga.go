// Package saga 分布式事务 saga：步骤编排 + 失败补偿。
package saga

import (
	"context"
	"errors"
	"sync"
)

// Step 是 saga 中的一个工作单元。
type Step struct {
	Name   string
	Do     func(ctx context.Context) error
	Undo   func(ctx context.Context) error
	Result any
}

// Outcome 是 saga 的记录结果。
type Outcome struct {
	Name     string
	Executed []string
	Error    error
	Undone   []string
	Rollback bool
}

// Coordinator 按顺序执行步骤；遇到首个错误时
// it runs each previously executed step's Undo in reverse
// order to compensate.
type Coordinator struct {
	mu       sync.Mutex
	onRollback func(o Outcome)
}

// New 返回一个空 coordinator。
func New() *Coordinator {
	return &Coordinator{}
}

// OnRollback 注册回滚完成后触发的钩子。
func (c *Coordinator) OnRollback(fn func(o Outcome)) {
	c.mu.Lock()
	c.onRollback = fn
	c.mu.Unlock()
}

// Run 执行 saga。如果某一步失败，前面的步骤
// are compensated and the outcome 记录 Rollback=true.
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

// ErrEmpty 在 Run 收到零个步骤时返回。
var ErrEmpty = errors.New("saga has no steps")
