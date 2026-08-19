// Package cronx 提供 5 段 cron 表达式的最小化匹配：
// 分 时 日 月 周（Mon=1..Sun=7）；支持 * , - / 与数字。
package cronx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule 表示解析后的 cron 计划。
type Schedule struct {
	min  [60]bool
	hour [24]bool
	dom  [32]bool
	mon  [13]bool
	dow  [8]bool
}

// Parse 解析表达式，失败返回 error。
func Parse(expr string) (*Schedule, error) {
	s := &Schedule{}
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cronx: 需要 5 段")
	}
	if err := setField(s.min[:], fields[0], 0, 59); err != nil {
		return nil, err
	}
	if err := setField(s.hour[:], fields[1], 0, 23); err != nil {
		return nil, err
	}
	if err := setField(s.dom[:], fields[2], 1, 31); err != nil {
		return nil, err
	}
	if err := setField(s.mon[:], fields[3], 1, 12); err != nil {
		return nil, err
	}
	if err := setField(s.dow[:], fields[4], 0, 7); err != nil {
		return nil, err
	}
	if fields[4] == "0" {
		s.dow[7] = true // 兼容 0=7=周日
	}
	return s, nil
}

func setField(field []bool, spec string, min, max int) error {
	ranges := strings.Split(spec, ",")
	for _, r := range ranges {
		if err := setRange(field, r, min, max); err != nil {
			return err
		}
	}
	return nil
}

func setRange(field []bool, spec string, min, max int) error {
	step := 1
	r := spec
	if i := strings.Index(spec, "/"); i >= 0 {
		var err error
		step, err = strconv.Atoi(spec[i+1:])
		if err != nil || step <= 0 {
			return fmt.Errorf("cronx: 步长错误")
		}
		r = spec[:i]
	}
	var lo, hi int
	if r == "*" {
		lo, hi = min, max
	} else if i := strings.Index(r, "-"); i >= 0 {
		lo, _ = strconv.Atoi(r[:i])
		hi, _ = strconv.Atoi(r[i+1:])
	} else {
		v, err := strconv.Atoi(r)
		if err != nil {
			return err
		}
		lo, hi = v, v
	}
	if lo < min || hi > max || lo > hi {
		return fmt.Errorf("cronx: 范围错误")
	}
	for i := lo; i <= hi; i += step {
		field[i] = true
	}
	return nil
}

// Match 返回 t 是否命中计划。
func (s *Schedule) Match(t time.Time) bool {
	return s.min[t.Minute()] &&
		s.hour[t.Hour()] &&
		s.dom[t.Day()] &&
		s.mon[int(t.Month())] &&
		s.dow[int(t.Weekday())]
}

// Next 返回下一个匹配的 time（含 t 之后）。
func (s *Schedule) Next(t time.Time) time.Time {
	t = t.Add(time.Minute)
	for i := 0; i < 60*24*366*2; i++ {
		if s.Match(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}
