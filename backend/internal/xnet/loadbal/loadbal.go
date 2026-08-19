// Package loadbal 提供客户端负载均衡器：
// 支持轮询、加权轮询、随机三种策略。
package loadbal

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
)

// Strategy 是选择策略。
type Strategy int

const (
	RoundRobin Strategy = iota
	WeightedRoundRobin
	Random
)

// Host 是负载均衡的目标。
type Host struct {
	Addr   string
	Weight int
}

// Balancer 是负载均衡器。
type Balancer struct {
	mu       sync.RWMutex
	hosts    []Host
	strategy Strategy
	counter  atomic.Uint64
}

// ErrNoHosts 在没有可用主机时返回。
var ErrNoHosts = errors.New("loadbal: 没有可用主机")

// New 创建一个 Balancer。
func New(strategy Strategy, hosts []Host) *Balancer {
	b := &Balancer{strategy: strategy}
	b.Update(hosts)
	return b
}

// Update 更新主机列表。
func (b *Balancer) Update(hosts []Host) {
	b.mu.Lock()
	b.hosts = make([]Host, len(hosts))
	copy(b.hosts, hosts)
	b.mu.Unlock()
}

// Next 选下一个主机地址。
func (b *Balancer) Next() (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.hosts) == 0 {
		return "", ErrNoHosts
	}
	switch b.strategy {
	case Random:
		return b.hosts[rand.Intn(len(b.hosts))].Addr, nil
	case WeightedRoundRobin:
		return b.weighted(), nil
	case RoundRobin:
		fallthrough
	default:
		return b.roundRobin(), nil
	}
}

func (b *Balancer) roundRobin() string {
	n := b.counter.Add(1) - 1
	return b.hosts[int(n)%len(b.hosts)].Addr
}

func (b *Balancer) weighted() string {
	total := 0
	for _, h := range b.hosts {
		w := h.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	if total == 0 {
		return b.hosts[0].Addr
	}
	n := b.counter.Add(1) - 1
	mod := int(n % uint64(total))
	acc := 0
	for _, h := range b.hosts {
		w := h.Weight
		if w <= 0 {
			w = 1
		}
		acc += w
		if mod < acc {
			return h.Addr
		}
	}
	return b.hosts[len(b.hosts)-1].Addr
}

// Hosts 返回当前快照。
func (b *Balancer) Hosts() []Host {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Host, len(b.hosts))
	copy(out, b.hosts)
	return out
}

// Count 返回主机数。
func (b *Balancer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.hosts)
}
