// Package timermx 提供一个简单的定时任务管理器：
// 一次性延迟任务 + 周期任务。
package timermx

import (
	"sync"
	"time"
)

// Manager 维护多个 timer/wheel。
type Manager struct {
	mu     sync.Mutex
	tasks  map[int]*task
	seq    int
	stop   chan struct{}
	once   sync.Once
}

type task struct {
	id       int
	delay    time.Duration
	period   time.Duration
	fn       func()
	timer    *time.Timer
	ticker   *time.Ticker
	canceled bool
}

// New 创建一个 Manager。
func New() *Manager {
	m := &Manager{tasks: make(map[int]*task), stop: make(chan struct{})}
	return m
}

// After 在 d 之后执行 fn，返回 id。
func (m *Manager) After(d time.Duration, fn func()) int {
	m.mu.Lock()
	m.seq++
	t := &task{id: m.seq, delay: d, fn: fn}
	t.timer = time.AfterFunc(d, func() {
		m.mu.Lock()
		canceled := t.canceled
		delete(m.tasks, t.id)
		m.mu.Unlock()
		if !canceled {
			fn()
		}
	})
	m.tasks[m.seq] = t
	m.mu.Unlock()
	return m.seq
}

// Every 每 d 周期执行 fn，返回 id。
func (m *Manager) Every(d time.Duration, fn func()) int {
	m.mu.Lock()
	m.seq++
	t := &task{id: m.seq, period: d, fn: fn}
	t.ticker = time.NewTicker(d)
	m.tasks[m.seq] = t
	m.mu.Unlock()
	go func() {
		for range t.ticker.C {
			m.mu.Lock()
			canceled := t.canceled
			m.mu.Unlock()
			if canceled {
				return
			}
			fn()
		}
	}()
	return m.seq
}

// Cancel 取消一个任务。
func (m *Manager) Cancel(id int) {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if ok {
		t.canceled = true
		if t.timer != nil {
			t.timer.Stop()
		}
		if t.ticker != nil {
			t.ticker.Stop()
		}
		delete(m.tasks, id)
	}
	m.mu.Unlock()
}

// Len 返回当前任务数。
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tasks)
}

// Shutdown 停止所有任务。
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tasks {
		t.canceled = true
		if t.timer != nil {
			t.timer.Stop()
		}
		if t.ticker != nil {
			t.ticker.Stop()
		}
	}
	m.tasks = make(map[int]*task)
}
