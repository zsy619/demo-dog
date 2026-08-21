package scheduler

// loop.go:调度循环与任务执行。

import (
	"container/heap"
	"context"
	"fmt"
	"time"
)

// loop 是 Scheduler 的主循环:等待最近任务到期或被唤醒。
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

// runTask 安全地执行任务 fn:捕获 panic,不影响调度循环。
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
