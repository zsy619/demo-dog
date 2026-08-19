// Package health 提供服务健康检查汇总器：
// 注册若干 Checker，按需调用 Check 返回整体状态。
package health

import (
	"context"
	"sync"
	"time"
)

// Status 是健康状态枚举。
type Status int

const (
	StatusOK Status = iota
	StatusDegraded
	StatusDown
)

// String 序列化状态。
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusDegraded:
		return "degraded"
	case StatusDown:
		return "down"
	default:
		return "unknown"
	}
}

// Report 是单次检查结果。
type Report struct {
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Message string        `json:"message,omitempty"`
	Latency time.Duration `json:"latency"`
}

// Checker 是健康检查单元。
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// CheckerFunc 是 Checker 的函数适配器。
type CheckerFunc struct {
	N string
	F func(ctx context.Context) error
}

// Name 返回检查器名。
func (c CheckerFunc) Name() string { return c.N }

// Check 执行检查。
func (c CheckerFunc) Check(ctx context.Context) error { return c.F(ctx) }

// Manager 管理一组 Checker。
type Manager struct {
	mu       sync.RWMutex
	checkers map[string]Checker
}

// New 创建一个空 Manager。
func New() *Manager {
	return &Manager{checkers: make(map[string]Checker)}
}

// Register 注册一个 Checker。
func (m *Manager) Register(c Checker) {
	m.mu.Lock()
	m.checkers[c.Name()] = c
	m.mu.Unlock()
}

// Check 在 ctx 下并发执行所有检查。
func (m *Manager) Check(ctx context.Context) []Report {
	m.mu.RLock()
	list := make([]Checker, 0, len(m.checkers))
	for _, c := range m.checkers {
		list = append(list, c)
	}
	m.mu.RUnlock()
	reports := make([]Report, len(list))
	var wg sync.WaitGroup
	for i, c := range list {
		wg.Add(1)
		go func(i int, c Checker) {
			defer wg.Done()
			start := time.Now()
			err := c.Check(ctx)
			r := Report{Name: c.Name(), Latency: time.Since(start)}
			if err != nil {
				r.Status = StatusDown
				r.Message = err.Error()
			} else {
				r.Status = StatusOK
			}
			reports[i] = r
		}(i, c)
	}
	wg.Wait()
	return reports
}

// Overall 返回整体最坏状态。
func Overall(reports []Report) Status {
	current := StatusOK
	for _, r := range reports {
		if r.Status > current {
			current = r.Status
		}
	}
	return current
}
