// Package worksteal 提供简单工作窃取调度：
// 多 worker 各自维护 deque，空时从其它 worker 偷任务。
package worksteal

import (
	"sync"
	"sync/atomic"
)

// Job 是一次需要执行的工作。
type Job func()

// Scheduler 是一组 worker 的协调者。
type Scheduler struct {
	workers int
	queues  []*deque
	wg      sync.WaitGroup
	closed  atomic.Bool
}

type deque struct {
	mu    sync.Mutex
	tasks [][]Job
}

// New 创建一个包含 n 个 worker 的调度器。
func New(n int) *Scheduler {
	if n < 1 {
		n = 1
	}
	s := &Scheduler{workers: n, queues: make([]*deque, n)}
	for i := 0; i < n; i++ {
		s.queues[i] = &deque{}
		s.wg.Add(1)
		go s.run(i)
	}
	return s
}

// Submit 投递一个任务到首个 worker。
func (s *Scheduler) Submit(j Job) {
	if s.closed.Load() {
		return
	}
	s.queues[0].push([]Job{j})
}

// Close 停止所有 worker。
func (s *Scheduler) Close() {
	s.closed.Store(true)
	for _, q := range s.queues {
		q.push(nil)
	}
	s.wg.Wait()
}

func (s *Scheduler) run(idx int) {
	defer s.wg.Done()
	q := s.queues[idx]
	for {
		batch := q.pop()
		if batch == nil {
			// 偷
			stole := false
			for i := 0; i < s.workers; i++ {
				if i == idx {
					continue
				}
				if b := s.queues[i].steal(); b != nil {
					batch = b
					stole = true
					break
				}
			}
			if !stole {
				if s.closed.Load() {
					return
				}
				continue
			}
		}
		for _, j := range batch {
			if j == nil {
				continue
			}
			j()
		}
	}
}

func (d *deque) push(b []Job) {
	d.mu.Lock()
	d.tasks = append(d.tasks, b)
	d.mu.Unlock()
}

func (d *deque) pop() []Job {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.tasks) == 0 {
		return nil
	}
	b := d.tasks[len(d.tasks)-1]
	d.tasks = d.tasks[:len(d.tasks)-1]
	return b
}

func (d *deque) steal() []Job {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.tasks) == 0 {
		return nil
	}
	b := d.tasks[0]
	d.tasks = d.tasks[1:]
	return b
}
