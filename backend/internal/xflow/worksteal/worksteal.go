// Package worksteal 提供简单工作窃取调度：
// 多 worker 各自维护双端队列，空时从其它 worker 偷任务。
//
// 特性：
//   - Submit 轮询分发到各 worker
//   - Job panic 不影响 worker 持续运行
//   - Close 幂等且安全
//   - Stats 暴露计数
package worksteal

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Job 是一次需要执行的工作。
type Job func()

// Scheduler 是一组 worker 的协调者。
type Scheduler struct {
	workers  int
	queues   []*deque
	wg       sync.WaitGroup
	closed   atomic.Bool
	submits  atomic.Uint64
	executed atomic.Uint64
	panics   atomic.Uint64
	next     atomic.Uint64 // 轮询 Submit
}

type deque struct {
	mu        sync.Mutex
	tasks     [][]Job
	hasPoison atomic.Bool
}

// Stats 是运行时统计。
type Stats struct {
	Workers  int    `json:"workers"`
	Submits  uint64 `json:"submits"`
	Executed uint64 `json:"executed"`
	Panics   uint64 `json:"panics"`
	Closed   bool   `json:"closed"`
}

// New 创建一个包含 n 个 worker 的调度器（n < 1 视为 1）。
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

// Submit 投递一个任务；按轮询分配到 worker。
// 已关闭时返回 false。
func (s *Scheduler) Submit(j Job) bool {
	if j == nil || s.closed.Load() {
		return false
	}
	idx := int(s.next.Add(1)-1) % s.workers
	s.queues[idx].push([]Job{j})
	s.submits.Add(1)
	return true
}

// Close 停止所有 worker。幂等。
func (s *Scheduler) Close() {
	if s.closed.Swap(true) {
		return
	}
	for _, q := range s.queues {
		q.poison()
	}
	s.wg.Wait()
}

// Stats 返回统计快照。
func (s *Scheduler) Stats() Stats {
	return Stats{
		Workers:  s.workers,
		Submits:  s.submits.Load(),
		Executed: s.executed.Load(),
		Panics:   s.panics.Load(),
		Closed:   s.closed.Load(),
	}
}

func (s *Scheduler) run(idx int) {
	defer s.wg.Done()
	q := s.queues[idx]
	for {
		batch := q.pop()
		if batch == nil {
			if s.closed.Load() {
				return
			}
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
			s.runJob(j)
		}
	}
}

func (s *Scheduler) runJob(j Job) {
	defer func() {
		if r := recover(); r != nil {
			s.panics.Add(1)
			// panic 信息可用于日志，但这里仅计数
			_ = fmt.Sprintf("worksteal: panic: %v", r)
		}
	}()
	j()
	s.executed.Add(1)
}

func (d *deque) push(b []Job) {
	d.mu.Lock()
	d.tasks = append(d.tasks, b)
	d.mu.Unlock()
}

func (d *deque) pop() []Job {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.hasPoison.Load() && len(d.tasks) == 0 {
		return nil
	}
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

// poison 标记 deque 已中毒（关闭时使用）。
func (d *deque) poison() {
	d.hasPoison.Store(true)
}
