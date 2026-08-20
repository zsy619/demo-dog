// Package smoke 烟雾测试：批量发送预定义请求验证服务可用。
package smoke

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Harness 运行冒烟检查并报告结果。
type Harness struct {
	mu       sync.Mutex
	checks   []Check
	results  []Result
	out      io.Writer
	failFast bool
}

// Check 是一个命名的冒烟检查。
type Check struct {
	Name string
	Fn   func() error
}

// Result 是运行一次 Check 的结果。
type Result struct {
	Name    string
	OK      bool
	Err     error
	Elapsed time.Duration
}

// New 创建一个向 w 写入的 Harness。
func New(w io.Writer) *Harness {
	return &Harness{out: w}
}

// WithFailFast 在首次失败时中止 harness。
func (h *Harness) WithFailFast() *Harness {
	h.failFast = true
	return h
}

// Add 注册一个 check。
func (h *Harness) Add(name string, fn func() error) {
	h.mu.Lock()
	h.checks = append(h.checks, Check{Name: name, Fn: fn})
	h.mu.Unlock()
}

// Run 执行所有 check 并返回结果切片。
func (h *Harness) Run() []Result {
	var results []Result
	for _, c := range h.checks {
		start := time.Now()
		err := c.Fn()
		elapsed := time.Since(start)
		r := Result{Name: c.Name, OK: err == nil, Err: err, Elapsed: elapsed}
		results = append(results, r)
		h.mu.Lock()
		h.results = append(h.results, r)
		h.mu.Unlock()
		if h.failFast && err != nil {
			break
		}
	}
	return results
}

// Report 写入一份可读的报告。
func (h *Harness) Report() {
	fmt.Fprintln(h.out, "smoke harness report")
	fmt.Fprintln(h.out, strings.Repeat("-", 60))
	h.mu.Lock()
	results := append([]Result{}, h.results...)
	h.mu.Unlock()
	for _, r := range results {
		mark := "OK"
		if !r.OK {
			mark = "FAIL"
		}
		fmt.Fprintf(h.out, "%s  %s  (%s)\n", mark, r.Name, r.Elapsed)
		if r.Err != nil {
			fmt.Fprintf(h.out, "    %s\n", r.Err)
		}
	}
	fmt.Fprintln(h.out, strings.Repeat("-", 60))
	total := len(results)
	passed := 0
	for _, r := range results {
		if r.OK {
			passed++
		}
	}
	fmt.Fprintf(h.out, "%d/%d passed\n", passed, total)
}

// Summary 返回汇总的计数。
type Summary struct {
	Total  int
	Passed int
	Failed int
}

// Summary 返回汇总的计数。
func (h *Harness) Summary() Summary {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := Summary{Total: len(h.results)}
	for _, r := range h.results {
		if r.OK {
			s.Passed++
		} else {
			s.Failed++
		}
	}
	return s
}

// Assert 是一个小的断言辅助工具。
type Assert struct {
	t testingT
}

// testingT 是 *testing.T 所满足的最小接口。
type testingT interface {
	Errorf(format string, args ...any)
}

// NewAssert 包装一个 *testing.T。
func NewAssert(t testingT) *Assert {
	return &Assert{t: t}
}

// True 在 v 为 false 时失败。
func (a *Assert) True(v bool, msg string) {
	if !v {
		a.t.Errorf("expected true: %s", msg)
	}
}

// False 在 v 为 true 时失败。
func (a *Assert) False(v bool, msg string) {
	if v {
		a.t.Errorf("expected false: %s", msg)
	}
}

// Nil 在 err 不为 nil 时失败。
func (a *Assert) Nil(err error, msg string) {
	if err != nil {
		a.t.Errorf("expected nil: %s: %v", msg, err)
	}
}

// Equal 在 a != b 时失败。
func (a *Assert) Equal(a1, b1 any, msg string) {
	if fmt.Sprintf("%v", a1) != fmt.Sprintf("%v", b1) {
		a.t.Errorf("expected %v == %v: %s", a1, b1, msg)
	}
}

// NoErr 返回一个包装错误的函数。
func NoErr(fn func() error) error {
	return fn()
}

// ErrIs 当 target 位于错误链中时返回 true。
func ErrIs(err, target error) bool {
	return errors.Is(err, target)
}
