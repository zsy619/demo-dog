// Package scheduler 提供一个简单的间隔调度器，
// 它把一组 Job 按各自间隔在后台协程中触发。
package scheduler

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Job 表示一个可被调度的任务。
type Job struct {
	Name     string
	Interval time.Duration
	Fn       func()
}

// ErrNilFn 在 Fn 为空时返回。
var ErrNilFn = errors.New("scheduler: 任务函数为空")

// Scheduler 后台持有多个 Job，按各自 Interval 触发。
type Scheduler struct {
	mu     sync.Mutex
	jobs   []Job
	wg     sync.WaitGroup
	stop   chan struct{}
	run    atomic.Bool
	total  atomic.Uint64
}

// New 创建一个空 Scheduler。
func New() *Scheduler {
	return &Scheduler{stop: make(chan struct{})}
}

// Add 注册一个任务。Interval <= 0 默认 1 秒。
func (s *Scheduler) Add(j Job) error {
	if j.Fn == nil {
		return ErrNilFn
	}
	if j.Interval <= 0 {
		j.Interval = time.Second
	}
	if j.Name == "" {
		j.Name = "anon"
	}
	s.mu.Lock()
	s.jobs = append(s.jobs, j)
	s.mu.Unlock()
	return nil
}

// Start 启动调度循环。
func (s *Scheduler) Start() {
	if !s.run.CompareAndSwap(false, true) {
		return
	}
	for _, j := range s.snapshot() {
		s.wg.Add(1)
		go s.loop(j)
	}
}

// Stop 停止所有任务并等待。
func (s *Scheduler) Stop() {
	if !s.run.CompareAndSwap(true, false) {
		return
	}
	close(s.stop)
	s.wg.Wait()
	s.stop = make(chan struct{})
}

// Total 返回触发次数累计。
func (s *Scheduler) Total() uint64 { return s.total.Load() }

func (s *Scheduler) snapshot() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Job, len(s.jobs))
	copy(out, s.jobs)
	return out
}

func (s *Scheduler) loop(j Job) {
	defer s.wg.Done()
	t := time.NewTicker(j.Interval)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			safeCall(j.Fn)
			s.total.Add(1)
		}
	}
}

func safeCall(fn func()) {
	defer func() {
		_ = recover()
	}()
	fn()
}
