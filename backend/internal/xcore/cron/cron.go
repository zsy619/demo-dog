// Package cron 定时任务：解析 5 段 cron 表达式并周期触发任务。
package cron

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Schedule 是 5 段的 cron 表达式。
// 字段顺序：minute hour dom month dow（分钟 小时 日 月 星期）
type Schedule struct {
	Minute map[int]bool
	Hour   map[int]bool
	DOM    map[int]bool
	Month  map[int]bool
	DOW    map[int]bool
}

// Parse 解析一个 5 段 cron 表达式。支持 *、N、N-M、*/N 以及逗号列表。
func Parse(expr string) (*Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}
	s := &Schedule{
		Minute: map[int]bool{},
		Hour:   map[int]bool{},
		DOM:    map[int]bool{},
		Month:  map[int]bool{},
		DOW:    map[int]bool{},
	}
	bounds := []struct {
		lo, hi int
	}{
		{0, 59},
		{0, 23},
		{1, 31},
		{1, 12},
		{0, 6},
	}
	targets := []*map[int]bool{&s.Minute, &s.Hour, &s.DOM, &s.Month, &s.DOW}
	for i, f := range fields {
		if err := parseField(f, bounds[i].lo, bounds[i].hi, targets[i]); err != nil {
			return nil, fmt.Errorf("field %d: %w", i, err)
		}
	}
	return s, nil
}

func parseField(field string, lo, hi int, dst *map[int]bool) error {
	for _, part := range strings.Split(field, ",") {
		if err := parsePart(part, lo, hi, dst); err != nil {
			return err
		}
	}
	return nil
}

func parsePart(part string, lo, hi int, dst *map[int]bool) error {
	step := 1
	if i := strings.Index(part, "/"); i >= 0 {
		base := part[:i]
		if base != "*" {
			return errors.New("step only allowed with *")
		}
		n, err := strconv.Atoi(part[i+1:])
		if err != nil || n <= 0 {
			return fmt.Errorf("bad step: %s", part)
		}
		step = n
	}
	start, end := lo, hi
	if i := strings.Index(part, "-"); i >= 0 {
		s, err := strconv.Atoi(part[:i])
		if err != nil {
			return err
		}
		e, err := strconv.Atoi(part[i+1:])
		if err != nil {
			return err
		}
		if s < lo || e > hi || s > e {
			return fmt.Errorf("bad range: %s", part)
		}
		start, end = s, e
	} else if part != "*" && !strings.Contains(part, "/") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("bad number: %s", part)
		}
		if n < lo || n > hi {
			return fmt.Errorf("out of range: %d", n)
		}
		(*dst)[n] = true
		return nil
	}
	for v := start; v <= end; v += step {
		(*dst)[v] = true
	}
	return nil
}

// Match 在 t 满足 schedule 时返回 true。
func (s *Schedule) Match(t time.Time) bool {
	if !s.Minute[t.Minute()] {
		return false
	}
	if !s.Hour[t.Hour()] {
		return false
	}
	if !s.Month[int(t.Month())] {
		return false
	}
	if !s.DOM[t.Day()] && !s.DOW[int(t.Weekday())] {
		return false
	}
	return true
}

// Next 返回 schedule 匹配的下一次时间，严格在 t 之后。
func (s *Schedule) Next(t time.Time) time.Time {
	cur := t.Add(time.Minute)
	cur = time.Date(cur.Year(), cur.Month(), cur.Day(), cur.Hour(), cur.Minute(), 0, 0, cur.Location())
	for i := 0; i < 366*24*60; i++ {
		if s.Match(cur) {
			return cur
		}
		cur = cur.Add(time.Minute)
	}
	return time.Time{}
}

// Task 表示一个具名定时任务。
type Task struct {
	Name string
	Expr string
	sched *Schedule
	lastRun time.Time
	run func(time.Time)
}

// Scheduler 持有定时任务集合。
type Scheduler struct {
	mu    sync.Mutex
	tasks []*Task
	now   func() time.Time
}

// New 构造一个空的 Scheduler。
func New() *Scheduler {
	return &Scheduler{now: time.Now}
}

// WithTime 覆盖时间源（用于测试）。
func (s *Scheduler) WithTime(now func() time.Time) *Scheduler {
	s.now = now
	return s
}

// Add 注册一个新任务。
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

// Tick 评估每个任务并触发到期任务，返回触发的任务数量。
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
