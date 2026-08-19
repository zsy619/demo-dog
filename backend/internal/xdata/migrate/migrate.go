// Package migrate 提供一个 KV 数据迁移协调器。
// 它按顺序执行一组 Migration，记录已完成索引，支持回滚最近一步。
package migrate

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Migration 表示一次迁移步骤。
type Migration struct {
	Name string
	Up   func(ctx context.Context, m *Migration) error
	Down func(ctx context.Context, m *Migration) error
}

// Record 表示已执行的迁移。
type Record struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Applied bool   `json:"applied"`
}

// Store 是迁移记录存储。
type Store interface {
	List(ctx context.Context) ([]Record, error)
	Append(ctx context.Context, rec Record) error
	Pop(ctx context.Context) (Record, error)
}

// ErrEmpty 在没有可执行迁移时返回。
var ErrEmpty = errors.New("migrate: 已无迁移可执行")

// Migrator 协调一组迁移的执行与回滚。
type Migrator struct {
	mu     sync.Mutex
	list   []Migration
	store  Store
	applied int
}

// New 创建一个 Migrator。
func New(store Store, list []Migration) *Migrator {
	return &Migrator{list: list, store: store}
}

// Apply 执行所有未完成的迁移。
func (m *Migrator) Apply(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	records, _ := m.store.List(ctx)
	next := len(records)
	for i := next; i < len(m.list); i++ {
		step := m.list[i]
		if err := step.Up(ctx, &step); err != nil {
			return fmt.Errorf("migrate: 第 %d 步 %s 失败: %w", i, step.Name, err)
		}
		if err := m.store.Append(ctx, Record{Index: i, Name: step.Name, Applied: true}); err != nil {
			return err
		}
		m.applied = i + 1
	}
	return nil
}

// Rollback 回滚最近一次已应用迁移。
func (m *Migrator) Rollback(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, err := m.store.Pop(ctx)
	if err != nil {
		return err
	}
	if rec.Index < 0 || rec.Index >= len(m.list) {
		return fmt.Errorf("migrate: 索引越界 %d", rec.Index)
	}
	step := m.list[rec.Index]
	if step.Down == nil {
		return fmt.Errorf("migrate: %s 未提供 Down", step.Name)
	}
	if err := step.Down(ctx, &step); err != nil {
		return err
	}
	m.applied = rec.Index
	return nil
}

// Applied 返回已应用的迁移数量。
func (m *Migrator) Applied() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applied
}

// Pending 返回剩余待执行的迁移数量。
func (m *Migrator) Pending() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.list) - m.applied
}

// MemStore 是一个简单的内存 Store 实现。
type MemStore struct {
	mu      sync.Mutex
	records []Record
}

// NewMemStore 创建一个空内存存储。
func NewMemStore() *MemStore { return &MemStore{} }

// List 返回所有记录副本。
func (s *MemStore) List(_ context.Context) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, len(s.records))
	copy(out, s.records)
	return out, nil
}

// Append 追加一条记录。
func (s *MemStore) Append(_ context.Context, rec Record) error {
	s.mu.Lock()
	s.records = append(s.records, rec)
	s.mu.Unlock()
	return nil
}

// Pop 弹出最后一条记录。
func (s *MemStore) Pop(_ context.Context) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) == 0 {
		return Record{}, ErrEmpty
	}
	last := s.records[len(s.records)-1]
	s.records = s.records[:len(s.records)-1]
	return last, nil
}
