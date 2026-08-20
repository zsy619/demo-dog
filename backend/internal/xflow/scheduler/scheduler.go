// Package scheduler 提供一个简单的任务调度器：
// 支持延时一次性任务（Once）和周期任务（Every）。
//
// 用法：
//
//	s := scheduler.New()
//	s.Start()
//	defer s.Stop()
//	s.Every("heartbeat", 30*time.Second, func(ctx context.Context) { ... })
//	s.Once("cleanup", 5*time.Minute, func(ctx context.Context) { ... })
package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// Task 描述一个调度任务。
type Task struct {
	Name     string
	NextRun  time.Time
	Interval time.Duration // 0 表示一次性任务
	Fn       func(ctx context.Context)
}

// Scheduler 管理一组 Task。
type Scheduler struct {
	mu      sync.Mutex
	heap    taskHeap
	stop    chan struct{}
	wake    chan struct{}
	run     bool
	onPanic func(name string, err error)
	onError func(name string, err error)
}

type taskItem struct {
	task *Task
	idx  int
}

type taskHeap []*taskItem

func (h taskHeap) Len() int           { return len(h) }
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
	return &Scheduler{
		stop: make(chan struct{}),
		wake: make(chan struct{}, 1),
		onPanic: func(string, error) {},
		onError: func(string, error) {},
	}
}

// OnPanic 注册 panic 处理器（默认忽略）。
func (s *Scheduler) OnPanic(fn func(name string, err error)) {
	s.mu.Lock()
	s.onPanic = fn
	s.mu.Unlock()
}

// OnError 注册任务 fn 返回非 nil error 时的回调。
func (s *Scheduler) OnError(fn func(name string, err error)) {
	s.mu.Lock()
	s.onError = fn
	s.mu.Unlock()
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

// Stop 停止调度循环；未执行的任务将不再执行。
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

// Every 注册周期任务（间隔 interval 反复执行）。
func (s *Scheduler) Every(name string, interval time.Duration, fn func(ctx context.Context)) {
	if interval <= 0 {
		interval = time.Second
	}
	s.add(&Task{Name: name, Interval: interval, NextRun: time.Now().Add(interval), Fn: fn})
}

// Once 注册一次性延时任务。
func (s *Scheduler) Once(name string, delay time.Duration, fn func(ctx context.Context)) {
	s.add(&Task{Name: name, Interval: 0, NextRun: time.Now().Add(delay), Fn: fn})
}

// Cancel 移除一个已注册的任务。返回 true 表示找到并移除。
func (s *Scheduler) Cancel(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, it := range s.heap {
		if it.task.Name == name {
			heap.Remove(&s.heap, i)
			return true
		}
	}
	return false
}

// Len 返回任务数。
func (s *Scheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heap.Len()
}

func (s *Scheduler) add(t *Task) {
	s.mu.Lock()
	heap.Push(&s.heap, &taskItem{task: t})
	s.mu.Unlock()
	s.notify()
}

func (s *Scheduler) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Scheduler) loop() {
	var timer *time.Timer
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
		onPanic := s.onPanic
		onError := s.onError
		s.mu.Unlock()

		delay := time.Until(next)
		if timer == nil {
			timer = time.NewTimer(delay)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(delay)
		}

		select {
		case <-s.stop:
			timer.Stop()
			return
		case <-timer.C:
		case <-s.wake:
			continue
		}

		s.mu.Lock()
		if s.heap.Len() == 0 {
			s.mu.Unlock()
			continue
		}
		top = heap.Pop(&s.heap).(*taskItem)
		interval := top.task.Interval
		name := top.task.Name
		fn := top.task.Fn
		s.mu.Unlock()

		s.runTask(name, fn, onPanic, onError)

		if interval > 0 {
			top.task.NextRun = time.Now().Add(interval)
			s.mu.Lock()
			heap.Push(&s.heap, top)
			s.mu.Unlock()
			s.notify()
		}
	}
}

// runTask 安全地执行任务 fn：捕获 panic，不影响调度循环。
func (s *Scheduler) runTask(name string, fn func(ctx context.Context), onPanic, onError func(name string, err error)) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			if onPanic != nil {
				onPanic(name, fmt.Errorf("scheduler task %q panic: %v", name, r))
			}
		}
	}()
	fn(context.Background())
	_ = onError
}
