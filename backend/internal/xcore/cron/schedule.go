// Package cron 定时任务:解析 5 段 cron 表达式并周期触发任务。
//
// 本包按类型拆分到多个文件:
//   - schedule.go  Schedule 表达式 + 解析/匹配
//   - scheduler.go Task + Scheduler 主体
package cron

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule 是 5 段的 cron 表达式。
// 字段顺序:minute hour dom month dow(分钟 小时 日 月 星期)
type Schedule struct {
	Minute map[int]bool
	Hour   map[int]bool
	DOM    map[int]bool
	Month  map[int]bool
	DOW    map[int]bool
}

// Parse 解析一个 5 段 cron 表达式。
//
// 支持 *, N, N-M, */N 以及逗号列表。
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

// parseField 按逗号拆分成多个 part 依次解析。
func parseField(field string, lo, hi int, dst *map[int]bool) error {
	for _, part := range strings.Split(field, ",") {
		if err := parsePart(part, lo, hi, dst); err != nil {
			return err
		}
	}
	return nil
}

// parsePart 解析单个 cron 表达式片段。
//
// 支持 *, N, N-M, */N;*/N 仅允许 * 作为基础。
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
//
// 注意:DOM 与 DOW 是 OR 关系(传统 Vixie cron 语义)。
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

// Next 返回 schedule 匹配的下一次时间,严格在 t 之后。
//
// 一年内未找到则返回 time.Time{}。
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
