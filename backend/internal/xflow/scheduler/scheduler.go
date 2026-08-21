// Package scheduler 提供一个简单的任务调度器:
// 支持延时一次性任务(Once)和周期任务(Every)。
//
// 文件职责拆分:
//   - scheduler.go Scheduler 主体 + 任务管理
//   - heap.go      任务堆实现
//   - loop.go      调度循环与任务执行
//
// 用法:
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
	"sync"
	"time"
)

// Task 描述一个调度任务。
type Task struct {
	Name     string           // 任务名
	NextRun  time.Time        // 下次执行时间
	Interval time.Duration    // 0 表示一次性任务
	Fn       func(ctx context.Context) // 任务函数
}

// Scheduler 管理一组 Task。
type Scheduler struct {
	mu      sync.Mutex                  // 保护 heap / 回调
	heap    taskHeap                    // 任务堆
	stop    chan struct{}               // 停止信号
	wake    chan struct{}               // 唤醒信号
	run     bool                        // 是否运行中
	onPanic func(name string, err error) // panic 回调
	onError func(name string, err error) // 错误回调
}

// New 创建一个 Scheduler。
func New() *Scheduler {
	return &Scheduler{
		stop:    make(chan struct{}),
		wake:    make(chan struct{}, 1),
		onPanic: func(string, error) {},
		onError: func(string, error) {},
	}
}

// OnPanic 注册 panic 处理器(默认忽略)。
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

// Stop 停止调度循环;未执行的任务将不再执行。
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

// Every 注册周期任务(间隔 interval 反复执行)。
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

// Cancel 移除一个已注册的任务;返回 true 表示找到并移除。
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

// add 将一个 Task 加入堆并唤醒调度循环。
func (s *Scheduler) add(t *Task) {
	s.mu.Lock()
	heap.Push(&s.heap, &taskItem{task: t})
	s.mu.Unlock()
	s.notify()
}

// notify 非阻塞地唤醒调度循环(已有唤醒信号则忽略)。
func (s *Scheduler) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
