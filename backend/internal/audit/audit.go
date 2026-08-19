package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Event is one audit record.
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

// Chain maintains a per-tenant tamper-evident audit log.
// Each event includes the SHA256 hash of the previous event
// so any tampering breaks the chain.
type Chain struct {
	mu       sync.Mutex
	tails    map[string]string   // tenant -> last hash
	seqs     map[string]uint64   // tenant -> last seq
	history  map[string][]*Event // tenant -> events
	now      func() time.Time
}

// New constructs an empty chain.
func New() *Chain {
	return &Chain{
		tails:   make(map[string]string),
		seqs:    make(map[string]uint64),
		history: make(map[string][]*Event),
		now:     time.Now,
	}
}

// WithTime overrides the time source for tests.
func (c *Chain) WithTime(now func() time.Time) *Chain {
	c.now = now
	return c
}

// Append records an event for tenant.
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

// Verify walks the chain for tenant and returns the index
// of the first broken event, or -1 if all OK.
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

// Events returns a copy of events for tenant.
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

// TenantSeq returns the last seq for tenant (0 if empty).
func (c *Chain) TenantSeq(tenant string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seqs[tenant]
}

// TenantTail returns the last hash for tenant ("" if empty).
func (c *Chain) TenantTail(tenant string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tails[tenant]
}

// ErrMissing is returned when no events exist for tenant.
var ErrMissing = errors.New("no events for tenant")

// VerifyAll verifies every tenant chain and returns the first
// broken tenant + index, or nil.
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
