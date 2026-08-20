// Package tpc 三方提交：协调 prepare/commit 阶段的一致性。
package tpc

import (
	"context"
	"errors"
	"sync"
)

// Phase is the current phase of a 2PC transaction.
type Phase string

const (
	PhaseInit    Phase = "init"
	PhasePrepare Phase = "prepare"
	PhaseCommit  Phase = "commit"
	PhaseAbort   Phase = "abort"
)

// Resource is a participant that can be prepared and
// committed (or rolled back).
type Resource struct {
	Name    string
	Prepare func(ctx context.Context) error
	Commit  func(ctx context.Context) error
	Abort   func(ctx context.Context) error
}

// Result describes the outcome.
type Result struct {
	Phase    Phase
	Error    error
	Prepared []string
	Committed []string
	Aborted  []string
}

// Coordinator runs two-phase commit.
type Coordinator struct {
	mu sync.Mutex
}

// New returns a Coordinator.
func New() *Coordinator { return &Coordinator{} }

// ErrAborted is the sentinel returned when the tx aborts.
var ErrAborted = errors.New("transaction aborted")

// Run runs PREPARE / COMMIT on every resource. If any
// Prepare fails, it runs Abort on all already-prepared
// resources.
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
