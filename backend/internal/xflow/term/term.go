// Package term 终止信号：监听 SIGTERM/SIGINT 触发优雅停机。
package term

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State 描述一个对端的集群视图。
type State struct {
	Term     uint64
	Leader   string
	IsLeader bool
	Peers    map[string]time.Time
}

// Clock 是 Raft 风格的 term 时钟。每个
// leader per term; followers promote themselves when their
// leader heartbeat expires.
type Clock struct {
	mu       sync.Mutex
	state    State
	self     string
	peerIDs  []string
	heartTTL time.Duration
	now      func() time.Time
	elects   atomic.Uint64
	stepdowns atomic.Uint64
	heartbeats atomic.Uint64
}

// New 为给定节点 ID 创建 Clock，并给定
// set of peer IDs (including self).
func New(self string, peers []string, heartTTL time.Duration) *Clock {
	if heartTTL <= 0 {
		heartTTL = 1 * time.Second
	}
	return &Clock{
		state: State{Peers: make(map[string]time.Time)},
		self:     self,
		peerIDs:  peers,
		heartTTL: heartTTL,
		now:      time.Now,
	}
}

// WithTime 覆盖测试的时间源。
func (c *Clock) WithTime(now func() time.Time) *Clock {
	c.now = now
	return c
}

// ErrStale 在来自更早 term 的请求
// arrives.
var ErrStale = errors.New("stale term")

// Heartbeat 更新 leader 状态。如果 term 更新
// than the current, demote self.
func (c *Clock) Heartbeat(from string, term uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if term < c.state.Term {
		return ErrStale
	}
	if term > c.state.Term {
		c.state.Term = term
		c.state.Leader = from
		c.state.IsLeader = false
		c.stepdowns.Add(1)
	}
	c.state.Peers[from] = c.now()
	c.heartbeats.Add(1)
	return nil
}

// MaybeElect 在当前 leader 未
// sent a heartbeat within heartTTL. The caller becomes
// leader for a new term.
func (c *Clock) MaybeElect() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.IsLeader {
		return false
	}
	if c.state.Leader != "" && c.state.Leader != c.self {
		last := c.state.Peers[c.state.Leader]
		if !last.IsZero() && c.now().Sub(last) < c.heartTTL {
			return false
		}
	}
	c.state.Term++
	c.state.Leader = c.self
	c.state.IsLeader = true
	c.state.Peers[c.self] = c.now()
	c.elects.Add(1)
	return true
}

// Snapshot 返回当前状态的副本。
func (c *Clock) Snapshot() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := State{
		Term: c.state.Term, Leader: c.state.Leader,
		IsLeader: c.state.IsLeader,
		Peers:    make(map[string]time.Time, len(c.state.Peers)),
	}
	for k, v := range c.state.Peers {
		s.Peers[k] = v
	}
	return s
}

// StepDown 在当前为 leader 时主动降级。
func (c *Clock) StepDown() {
	c.mu.Lock()
	if c.state.IsLeader {
		c.state.IsLeader = false
		c.stepdowns.Add(1)
	}
	c.mu.Unlock()
}

// Stats 返回计数器快照。
type Stats struct {
	Term       uint64 `json:"term"`
	Leader     string `json:"leader"`
	IsLeader   bool   `json:"is_leader"`
	Elections  uint64 `json:"elections"`
	Stepdowns  uint64 `json:"stepdowns"`
	Heartbeats uint64 `json:"heartbeats"`
}

// Stats 返回计数器快照。
func (c *Clock) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Term: c.state.Term, Leader: c.state.Leader,
		IsLeader:   c.state.IsLeader,
		Elections:  c.elects.Load(),
		Stepdowns:  c.stepdowns.Load(),
		Heartbeats: c.heartbeats.Load(),
	}
}
