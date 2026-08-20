package vclock

import (
	"sort"
	"sync"
)

// Clock is a vector clock: a mapping of node -> counter.
type Clock struct {
	mu    sync.RWMutex
	entry map[string]uint64
}

// New creates an empty Clock.
func New() *Clock {
	return &Clock{entry: make(map[string]uint64)}
}

// Set replaces the counter for node.
func (c *Clock) Set(node string, v uint64) {
	c.mu.Lock()
	c.entry[node] = v
	c.mu.Unlock()
}

// Tick increments the counter for self and returns the new
// value.
func (c *Clock) Tick(self string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entry[self]++
	return c.entry[self]
}

// Get returns the counter for node, or 0.
func (c *Clock) Get(node string) uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entry[node]
}

// Update merges other into c: takes the element-wise max.
// Returns the resulting "score" for ordering.
func (c *Clock) Update(other *Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	for n, v := range other.entry {
		if cur, ok := c.entry[n]; !ok || v > cur {
			c.entry[n] = v
		}
	}
}

// Compare returns the relationship between two clocks:
// -1 if c < other (c is causally before other),
//  0 if c == other (equal),
//  1 if c > other (c is causally after other),
//  2 if c and other are concurrent.
func (c *Clock) Compare(other *Clock) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	less, greater := false, false
	for n, v := range c.entry {
		o := other.entry[n]
		if v < o {
			less = true
		}
		if v > o {
			greater = true
		}
	}
	for n, v := range other.entry {
		if _, ok := c.entry[n]; !ok && v > 0 {
			less = true
		}
	}
	switch {
	case less && greater:
		return 2
	case less:
		return -1
	case greater:
		return 1
	default:
		return 0
	}
}

// Snapshot returns a stable copy of the clock state.
func (c *Clock) Snapshot() map[string]uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]uint64, len(c.entry))
	for k, v := range c.entry {
		out[k] = v
	}
	return out
}

// FromSnapshot restores from a map.
func (c *Clock) FromSnapshot(m map[string]uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entry = make(map[string]uint64, len(m))
	for k, v := range m {
		c.entry[k] = v
	}
}

// Nodes returns the sorted node list.
func (c *Clock) Nodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.entry))
	for n := range c.entry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
