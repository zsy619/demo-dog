// Package sempool 提供带权重的资源信号量池。
// 每个池条目代表一种资源条目（如一个连接槽位），
// Acquire 根据权重占用 N 个单位，Release 时归还。
package sempool

import (
	"errors"
	"sync"
)

// ErrFull 在所有条目权重不足时返回。
var ErrFull = errors.New("sempool: 容量耗尽")

// Slot 表示一种带权重的资源条目。
type Slot struct {
	Name   string
	Weight int
}

// Pool 维护一组 Slot，按请求权重挑选第一个能容纳的。
type Pool struct {
	mu    sync.Mutex
	slots []slot
}

type slot struct {
	weight int
	used   int
	name   string
}

// New 创建一个容量池，初始 slots 由调用方提供。
func New(slots []Slot) *Pool {
	p := &Pool{}
	for _, s := range slots {
		p.slots = append(p.slots, slot{weight: s.Weight, name: s.Name})
	}
	return p
}

// Acquire 占用权重为 n 的资源，返回 slot 名字与释放函数。
func (p *Pool) Acquire(n int) (string, func(), error) {
	if n <= 0 {
		return "", func() {}, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.slots {
		if p.slots[i].weight-p.slots[i].used >= n {
			p.slots[i].used += n
			idx := i
			return p.slots[i].name, func() {
				p.mu.Lock()
				p.slots[idx].used -= n
				p.mu.Unlock()
			}, nil
		}
	}
	return "", func() {}, ErrFull
}

// AddSlot 动态添加一个 slot。
func (p *Pool) AddSlot(s Slot) {
	p.mu.Lock()
	p.slots = append(p.slots, slot{weight: s.Weight, name: s.Name})
	p.mu.Unlock()
}

// Available 返回各 slot 的剩余容量。
func (p *Pool) Available() []Slot {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Slot, len(p.slots))
	for i, s := range p.slots {
		out[i] = Slot{Name: s.name, Weight: s.weight - s.used}
	}
	return out
}

// Total 返回所有 slot 总权重。
func (p *Pool) Total() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, s := range p.slots {
		n += s.weight
	}
	return n
}

// Used 返回所有 slot 已用权重。
func (p *Pool) Used() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, s := range p.slots {
		n += s.used
	}
	return n
}
