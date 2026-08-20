// Package topo 拓扑结构：表示节点/边的层级关系，支持序列化与遍历。
package topo

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Status is the lifecycle status of a task.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusDone
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	case StatusFailed:
		return "failed"
	}
	return "unknown"
}

// Task is one node in the topology.
type Task struct {
	ID       string
	Parent   string
	Children []string
	Status   Status
	Err      error
}

// Queue maintains a parent/child task topology. Children
// cannot run until their parent is Done.
type Queue struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	notReady []string // tasks waiting for parent (parent not Done)
	ready    chan string
	done     atomic.Uint64
	running  atomic.Int64
	closed   atomic.Uint32
}

// New creates an empty Queue.
func New() *Queue {
	return &Queue{tasks: make(map[string]*Task), ready: make(chan string, 1024)}
}

// Add registers a task. If a parent is set and not yet Done,
// the task is queued for parent completion.
func (q *Queue) Add(id, parent string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.tasks[id]; ok {
		return
	}
	t := &Task{ID: id, Parent: parent, Status: StatusPending}
	q.tasks[id] = t
	if parent != "" {
		if p, ok := q.tasks[parent]; ok {
			p.Children = append(p.Children, id)
		}
	}
	if parent == "" || (q.tasks[parent] != nil && q.tasks[parent].Status == StatusDone) {
		q.enqueueReady(id)
	}
}

func (q *Queue) enqueueReady(id string) {
	if q.closed.Load() == 0 {
		q.ready <- id
	}
}

// ErrNoTask is returned when the requested ID is unknown.
var ErrNoTask = errors.New("task not found")

// Start marks the task as running.
func (q *Queue) Start(id string) error {
	q.mu.Lock()
	t, ok := q.tasks[id]
	if !ok {
		q.mu.Unlock()
		return ErrNoTask
	}
	if t.Status != StatusPending {
		q.mu.Unlock()
		return errors.New("not pending")
	}
	t.Status = StatusRunning
	q.running.Add(1)
	q.mu.Unlock()
	return nil
}

// Complete marks the task as done and unblocks its children.
func (q *Queue) Complete(id string, err error) error {
	q.mu.Lock()
	t, ok := q.tasks[id]
	if !ok {
		q.mu.Unlock()
		return ErrNoTask
	}
	if err != nil {
		t.Status = StatusFailed
	} else {
		t.Status = StatusDone
		for _, c := range t.Children {
			q.enqueueReady(c)
		}
	}
	t.Err = err
	q.running.Add(-1)
	q.done.Add(1)
	q.mu.Unlock()
	return nil
}

// Next returns the next ready task ID, blocking until one is
// available or the queue is closed.
func (q *Queue) Next() (string, bool) {
	id, ok := <-q.ready
	return id, ok
}

// Get returns a snapshot of a task.
func (q *Queue) Get(id string) (*Task, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	t, ok := q.tasks[id]
	if !ok {
		return nil, false
	}
	cp := *t
	cp.Children = append([]string{}, t.Children...)
	return &cp, true
}

// Stats returns counters.
type Stats struct {
	Tasks   int    `json:"tasks"`
	Running int64  `json:"running"`
	Done    uint64 `json:"done"`
	Failed  int    `json:"failed"`
}

// Stats returns the snapshot.
func (q *Queue) Stats() Stats {
	q.mu.RLock()
	defer q.mu.RUnlock()
	stats := Stats{Tasks: len(q.tasks), Running: q.running.Load(), Done: q.done.Load()}
	for _, t := range q.tasks {
		if t.Status == StatusFailed {
			stats.Failed++
		}
	}
	return stats
}

// Close shuts down the queue. No further tasks can be added.
func (q *Queue) Close() {
	if q.closed.CompareAndSwap(0, 1) {
		close(q.ready)
	}
}
