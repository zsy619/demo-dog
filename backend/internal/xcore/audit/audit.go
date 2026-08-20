// Package audit 审计事件日志：记录关键操作，支持查询与过滤。
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Event 表示一条审计记录。
type Event struct {
	Tenant    string
	Action    string
	Actor     string
	Target    string
	Metadata  map[string]string
	At        time.Time
	PrevHash  string
	Hash      string
	Seq       uint64
}

// Chain 为每个租户维护一条防篡改审计日志。
// 每条事件都包含前一条事件的 SHA256 校验和，
// 任何篡改都会导致链断裂。
type Chain struct {
	mu       sync.Mutex
	tails    map[string]string   // tenant -> last hash
	seqs     map[string]uint64   // tenant -> last seq
	history  map[string][]*Event // tenant -> events
	now      func() time.Time
}

// New 构造一个空的 Chain。
func New() *Chain {
	return &Chain{
		tails:   make(map[string]string),
		seqs:    make(map[string]uint64),
		history: make(map[string][]*Event),
		now:     time.Now,
	}
}

// WithTime 覆盖时间源（用于测试）。
func (c *Chain) WithTime(now func() time.Time) *Chain {
	c.now = now
	return c
}

// Append 为租户记录一条事件。
func (c *Chain) Append(tenant, action, actor, target string, meta map[string]string) *Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev := c.tails[tenant]
	seq := c.seqs[tenant] + 1
	e := &Event{
		Tenant: tenant, Action: action, Actor: actor,
		Target: target, Metadata: meta,
		At: c.now(), PrevHash: prev, Seq: seq,
	}
	e.Hash = hashEvent(e)
	c.tails[tenant] = e.Hash
	c.seqs[tenant] = seq
	c.history[tenant] = append(c.history[tenant], e)
	return e
}

// Verify 检查指定租户的链，返回第一个被破坏的事件下标；
// 若全部正常则返回 -1。
func (c *Chain) Verify(tenant string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	events := c.history[tenant]
	var prev string
	for i, e := range events {
		if e.PrevHash != prev {
			return i
		}
		if e.Hash != hashEvent(e) {
			return i
		}
		prev = e.Hash
	}
	return -1
}

// Events 返回指定租户事件的副本。
func (c *Chain) Events(tenant string) []*Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	events := c.history[tenant]
	out := make([]*Event, len(events))
	for i, e := range events {
		cp := *e
		history := c.history[tenant]
		_ = history
		if e.Metadata != nil {
			cp.Metadata = make(map[string]string, len(e.Metadata))
			for k, v := range e.Metadata {
				cp.Metadata[k] = v
			}
		}
		out[i] = &cp
	}
	return out
}

// TenantSeq 返回指定租户最后的事件序号（若为空则为 0）。
func (c *Chain) TenantSeq(tenant string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seqs[tenant]
}

// TenantTail 返回指定租户最后的事件哈希（若为空则为 ""）。
func (c *Chain) TenantTail(tenant string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tails[tenant]
}

// ErrMissing 在租户没有任何事件时返回。
var ErrMissing = errors.New("no events for tenant")

// VerifyAll 校验所有租户的链，返回首个被破坏的租户与下标；
// 若全部正常则返回 nil。
func (c *Chain) VerifyAll() map[string]int {
	c.mu.Lock()
	tenants := make([]string, 0, len(c.history))
	for t := range c.history {
		tenants = append(tenants, t)
	}
	c.mu.Unlock()
	out := make(map[string]int)
	for _, t := range tenants {
		if idx := c.Verify(t); idx >= 0 {
			out[t] = idx
		}
	}
	return out
}

func hashEvent(e *Event) string {
	h := sha256.New()
	h.Write([]byte(e.Tenant))
	h.Write([]byte("|"))
	h.Write([]byte(e.Action))
	h.Write([]byte("|"))
	h.Write([]byte(e.Actor))
	h.Write([]byte("|"))
	h.Write([]byte(e.Target))
	h.Write([]byte("|"))
	h.Write([]byte(e.At.Format(time.RFC3339Nano)))
	h.Write([]byte("|"))
	for _, k := range sortedKeys(e.Metadata) {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(e.Metadata[k]))
		h.Write([]byte(";"))
	}
	h.Write([]byte("|"))
	h.Write([]byte(e.PrevHash))
	var seqBuf [8]byte
	seq := e.Seq
	for i := 7; i >= 0; i-- {
		seqBuf[i] = byte(seq)
		seq >>= 8
	}
	h.Write(seqBuf[:])
	return hex.EncodeToString(h.Sum(nil))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
