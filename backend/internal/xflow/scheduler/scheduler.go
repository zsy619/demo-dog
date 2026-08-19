// Package scheduler 提供一个简单的任务调度器：
// 支持延时 + 周期任务。
package scheduler

import (
	"container/heap"
	"sync"
	"time"
)

// Task 描述一个调度任务。
type Task struct {
	Name     string
	NextRun  time.Time
	Interval time.Duration
	Fn       func()
}

// Scheduler 管理一组 Task。
type Scheduler struct {
	mu    sync.Mutex
	heap  taskHeap
	stop  chan struct{}
	wake  chan struct{}
	run   bool
}

type taskItem struct {
	task *Task
	idx  int
}

type taskHeap []*taskItem

func (h taskHeap) Len() int { return len(h) }
func (h taskHeap) Less(i, j int) bool { return h[i].task.NextRun.Before(h[j].task.NextRun) }
func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].idx = i
	h[j].idx = j
}
func (h *taskHeap) Push(x any) {
	t := x.(*taskItem)
	t.idx = len(*h)
	*h = append(*h, t)
}
func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}

// New 创建一个 Scheduler。
func New() *Scheduler {
	return &Scheduler{stop: make(chan struct{}), wake: make(chan struct{}, 1)}
}

// Start 启动调度循环。
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.run {
		s.mu.Unlock()
		return
	}
	s.run = true
	s.mu.Unlock()
	go s.loop()
}

// Stop 停止调度循环。
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.run {
		s.mu.Unlock()
		return
	}
	s.run = false
	s.mu.Unlock()
	close(s.stop)
}

// Add 注册一个周期任务。
func (s *Scheduler) Add(name string, interval time.Duration, fn func()) {
	t := &Task{Name: name, Interval: interval, NextRun: time.Now().Add(interval), Fn: fn}
	s.mu.Lock()
	heap.Push(&s.heap, &taskItem{task: t})
	s.mu.Unlock()
	s.notify()
}

// Once 注册一个延时任务。
func (s *Scheduler) Once(name string, delay time.Duration, fn func()) {
	t := &Task{Name: name, Interval: 0, NextRun: time.Now().Add(delay), Fn: fn}
	s.mu.Lock()
	heap.Push(&s.heap, &taskItem{task: t})
	s.mu.Unlock()
	s.notify()
}

// Len 返回任务数。
func (s *Scheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heap.Len()
}

func (s *Scheduler) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Scheduler) loop() {
	t := time.NewTimer(time.Hour)
	defer t.Stop()
	for {
		s.mu.Lock()
		if s.heap.Len() == 0 {
			s.mu.Unlock()
			select {
			case <-s.stop:
				return
			case <-s.wake:
				continue
			}
		}
		top := s.heap[0]
		next := top.task.NextRun
		s.mu.Unlock()
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		t.Reset(time.Until(next))
		select {
		case <-s.stop:
			return
		case <-t.C:
		case <-s.wake:
		}
		s.mu.Lock()
		if s.heap.Len() == 0 {
			s.mu.Unlock()
			continue
		}
		top = heap.Pop(&s.heap).(*taskItem)
		s.mu.Unlock()
		top.task.Fn()
		if top.task.Interval > 0 {
			top.task.NextRun = time.Now().Add(top.task.Interval)
			s.mu.Lock()
			heap.Push(&s.heap, top)
			s.mu.Unlock()
		}
	}
}
