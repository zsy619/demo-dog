package cron

// scheduler.go:Task 与 Scheduler 主体。
//
// Scheduler 持有任务列表并周期 Tick() 触发到期任务;
// 每个 Task 内部缓存 lastRun 时间以避免同一分钟内重复触发。

import (
	"sync"
	"time"
)

// Task 表示一个具名定时任务。
type Task struct {
	Name    string         // 任务名(用于 List/Add 查找)
	Expr    string         // 原始 cron 表达式(保留便于显示)
	sched   *Schedule      // 解析后的调度
	lastRun time.Time      // 最近一次执行时间(去重)
	run     func(time.Time) // 实际执行回调
}

// Scheduler 持有定时任务集合。
type Scheduler struct {
	mu    sync.Mutex    // 保护 tasks
	tasks []*Task       // 已注册任务
	now   func() time.Time // 时间源(测试可注入)
}

// New 构造一个空的 Scheduler。
func New() *Scheduler {
	return &Scheduler{now: time.Now}
}

// WithTime 覆盖时间源(用于测试)。
func (s *Scheduler) WithTime(now func() time.Time) *Scheduler {
	s.now = now
	return s
}

// Add 注册一个新任务。
//
// expr 必须能被 Parse 接受;否则返回错误。
func (s *Scheduler) Add(name, expr string, run func(time.Time)) error {
	sched, err := Parse(expr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.tasks = append(s.tasks, &Task{Name: name, Expr: expr, sched: sched, run: run})
	s.mu.Unlock()
	return nil
}

// Tick 评估每个任务并触发到期任务,返回触发的任务数量。
//
// 通过 lastRun 字段避免在同一分钟内重复触发。
func (s *Scheduler) Tick() int {
	s.mu.Lock()
	tasks := make([]*Task, len(s.tasks))
	copy(tasks, s.tasks)
	s.mu.Unlock()
	now := s.now()
	n := 0
	for _, t := range tasks {
		if t.sched.Match(now) && !t.lastRun.Equal(now) {
			t.run(now)
			t.lastRun = now
			n++
		}
	}
	return n
}

// List 返回所有任务的名称。
func (s *Scheduler) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t.Name)
	}
	return out
}
