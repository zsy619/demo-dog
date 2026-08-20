// Package tpc 三方提交：协调 prepare/commit 阶段的一致性。
package tpc

import (
	"context"
	"errors"
	"sync"
)

// Phase 表示 2PC 事务的当前阶段。
type Phase string

const (
	PhaseInit    Phase = "init"
	PhasePrepare Phase = "prepare"
	PhaseCommit  Phase = "commit"
	PhaseAbort   Phase = "abort"
)

// Resource 是一个可以 prepare 和 commit (或 rollback) 的参与者。
// Resource 是一个可以 prepare 和 commit (或 rollback) 的参与者。
type Resource struct {
	Name    string
	Prepare func(ctx context.Context) error
	Commit  func(ctx context.Context) error
	Abort   func(ctx context.Context) error
}

// Result 描述执行结果。
type Result struct {
	Phase    Phase
	Error    error
	Prepared []string
	Committed []string
	Aborted  []string
}

// Coordinator 运行两阶段提交。
type Coordinator struct {
	mu sync.Mutex
}

// New 返回一个 Coordinator。
func New() *Coordinator { return &Coordinator{} }

// ErrAborted 是事务中止时返回的哨兵错误。
var ErrAborted = errors.New("transaction aborted")

// Run 对每个 resource 运行 PREPARE / COMMIT。如果任何 Prepare 失败，
// 会对所有已经 prepare 的 resource 运行 Abort。
// 会对所有已经 prepare 的 resource 运行 Abort。
func (c *Coordinator) Run(ctx context.Context, resources []Resource) Result {
	res := Result{Phase: PhaseInit}
	if len(resources) == 0 {
		res.Error = errors.New("no resources")
		return res
	}
	res.Phase = PhasePrepare
	for _, r := range resources {
		if err := ctx.Err(); err != nil {
			res.Error = err
			c.abortAll(ctx, resources[:len(res.Prepared)], &res)
			return res
		}
		if err := r.Prepare(ctx); err != nil {
			res.Error = err
			c.abortAll(ctx, resources[:len(res.Prepared)], &res)
			return res
		}
		res.Prepared = append(res.Prepared, r.Name)
	}
	res.Phase = PhaseCommit
	for _, r := range resources {
		if err := r.Commit(ctx); err != nil {
			res.Error = err
			res.Phase = PhaseAbort
			return res
		}
		res.Committed = append(res.Committed, r.Name)
	}
	return res
}

func (c *Coordinator) abortAll(ctx context.Context, resources []Resource, res *Result) {
	res.Phase = PhaseAbort
	for _, r := range resources {
		if r.Abort == nil {
			continue
		}
		_ = r.Abort(ctx)
		res.Aborted = append(res.Aborted, r.Name)
	}
}
