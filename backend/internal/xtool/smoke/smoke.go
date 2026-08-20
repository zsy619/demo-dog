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

// Harness runs smoke checks and reports results.
type Harness struct {
	mu       sync.Mutex
	checks   []Check
	results  []Result
	out      io.Writer
	failFast bool
}

// Check is one named smoke check.
type Check struct {
	Name string
	Fn   func() error
}

// Result is the outcome of running one Check.
type Result struct {
	Name    string
	OK      bool
	Err     error
	Elapsed time.Duration
}

// New creates a Harness writing to w.
func New(w io.Writer) *Harness {
	return &Harness{out: w}
}

// WithFailFast aborts the harness on the first failure.
func (h *Harness) WithFailFast() *Harness {
	h.failFast = true
	return h
}

// Add registers a check.
func (h *Harness) Add(name string, fn func() error) {
	h.mu.Lock()
	h.checks = append(h.checks, Check{Name: name, Fn: fn})
	h.mu.Unlock()
}

// Run executes all checks and returns a slice of results.
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

// Report writes a human-readable report.
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

// Summary returns aggregate counts.
type Summary struct {
	Total  int
	Passed int
	Failed int
}

// Summary returns the aggregate counts.
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

// Assert is a small assertion helper.
type Assert struct {
	t testingT
}

// testingT is the minimal interface *testing.T satisfies.
type testingT interface {
	Errorf(format string, args ...any)
}

// NewAssert wraps a *testing.T.
func NewAssert(t testingT) *Assert {
	return &Assert{t: t}
}

// True fails if v is false.
func (a *Assert) True(v bool, msg string) {
	if !v {
		a.t.Errorf("expected true: %s", msg)
	}
}

// False fails if v is true.
func (a *Assert) False(v bool, msg string) {
	if v {
		a.t.Errorf("expected false: %s", msg)
	}
}

// Nil fails if err is not nil.
func (a *Assert) Nil(err error, msg string) {
	if err != nil {
		a.t.Errorf("expected nil: %s: %v", msg, err)
	}
}

// Equal fails if a != b.
func (a *Assert) Equal(a1, b1 any, msg string) {
	if fmt.Sprintf("%v", a1) != fmt.Sprintf("%v", b1) {
		a.t.Errorf("expected %v == %v: %s", a1, b1, msg)
	}
}

// NoErr returns an error-wrapping fn.
func NoErr(fn func() error) error {
	return fn()
}

// ErrIs returns true if target is in chain.
func ErrIs(err, target error) bool {
	return errors.Is(err, target)
}
