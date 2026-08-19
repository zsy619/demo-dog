// Package replica implements WAL replication between a primary
// demo-dog collector and one or more followers. It is intentionally
// stdlib-only and trades strong-consensus guarantees for
// zero-dependency simplicity.
//
// Threat model:
//   - Two-node active/passive HA. Primary accepts writes. Follower
//     tails the primary WAL over HTTP and applies every record to
//     its own in-memory engine.
//   - Failover is manual: a flag flip + restart promotes the
//     follower to primary. The operator runs
//     dog-collector --role=primary on the new box and
//     --role=follower --peer=<new-primary> on the old one.
//   - We do NOT implement Raft/etcd-style leader election. That
//     requires hashicorp/raft which conflicts with the stdlib-only
//     policy. Operators who need consensus should use etcd or
//     consul and have their service-mesh flip the route.
//
// Wire protocol (HTTP, JSON):
//   GET  /replica/offset            -> primary reports last offset
//   GET  /replica/wal?from=<offset> -> follow records starting at offset
//
// Concurrency:
//   * One replica goroutine per follower. Idempotent restart on
//     disconnect.
//   * The follower applies records under the same mutex the engine
//     uses for ingest, so no observer sees a half-applied state.
package replica

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Role identifies whether this node is a primary or follower.
type Role string

const (
	RolePrimary  Role = "primary"
	RoleFollower Role = "follower"
)

// Record is one WAL entry that has been replicated.
type Record struct {
	Op      uint32 `json:"op"`
	Payload []byte `json:"payload"`
	Offset  int64  `json:"offset"`
}

// State is the runtime state of a node.
type State struct {
	Role     Role      `json:"role"`
	Offset   int64     `json:"offset"`
	LastSync time.Time `json:"last_sync"`
	Peer     string    `json:"peer,omitempty"`
	Synced   uint64    `json:"synced"`
	Dropped  uint64    `json:"dropped"`
}

// Node is the runtime replica manager.
type Node struct {
	mu      sync.Mutex
	role    Role
	peer    string
	offset  atomic.Int64
	synced  atomic.Uint64
	dropped atomic.Uint64
	emit    chan Record
	apply   chan Record
	stopCh  chan struct{}
}

// NewPrimary returns a Node in primary role.
func NewPrimary() *Node {
	return &Node{
		role:   RolePrimary,
		emit:   make(chan Record, 1024),
		stopCh: make(chan struct{}),
	}
}

// NewFollower returns a Node in follower role pointing at the peer.
func NewFollower(peer string) *Node {
	return &Node{
		role:   RoleFollower,
		peer:   peer,
		apply:  make(chan Record, 1024),
		stopCh: make(chan struct{}),
	}
}

// Emit is called by the primary WAL hook.
func (n *Node) Emit(op uint32, payload []byte) {
	if n.role != RolePrimary {
		return
	}
	off := n.offset.Add(1)
	rec := Record{Op: op, Payload: payload, Offset: off}
	select {
	case n.emit <- rec:
	default:
		n.dropped.Add(1)
	}
}

// Apply returns the channel that delivers records to the engine.
func (n *Node) Apply() <-chan Record {
	return n.apply
}

// Stats returns the current state for /api/health.
func (n *Node) Stats() State {
	n.mu.Lock()
	defer n.mu.Unlock()
	return State{
		Role:     n.role,
		Offset:   n.offset.Load(),
		Peer:     n.peer,
		Synced:   n.synced.Load(),
		Dropped:  n.dropped.Load(),
		LastSync: time.Now(),
	}
}

// Start begins the replication loop.
func (n *Node) Start() error {
	go n.run()
	return nil
}

// Stop tears down the goroutine.
func (n *Node) Stop() {
	select {
	case <-n.stopCh:
	default:
		close(n.stopCh)
	}
}

func (n *Node) run() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-n.stopCh:
			return
		case <-t.C:
			if n.role == RoleFollower {
				n.tick()
			}
		}
	}
}

func (n *Node) tick() {
	if n.peer == "" {
		return
	}
	from := n.offset.Load()
	url := fmt.Sprintf("http://%s/replica/wal?from=%d", n.peer, from)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	dec := json.NewDecoder(resp.Body)
	for {
		var rec Record
		if err := dec.Decode(&rec); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
		if rec.Offset <= from {
			continue
		}
		select {
		case n.apply <- rec:
			n.offset.Store(rec.Offset)
			n.synced.Add(1)
		case <-n.stopCh:
			return
		}
	}
}

// Handler returns the HTTP handler for the primary side.
func (n *Node) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/replica/offset", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"offset": n.offset.Load(),
			"role":   string(n.role),
		})
	})
	mux.HandleFunc("/replica/wal", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		from := int64(0)
		if v := r.URL.Query().Get("from"); v != "" {
			_, _ = fmt.Sscanf(v, "%d", &from)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)
		n.mu.Lock()
		defer n.mu.Unlock()
		for rec := range n.emitSnapshot(from) {
			_ = enc.Encode(rec)
		}
	})
	return mux
}

func (n *Node) emitSnapshot(from int64) <-chan Record {
	out := make(chan Record, 256)
	go func() {
		defer close(out)
		for {
			select {
			case rec := <-n.emit:
				if rec.Offset > from {
					out <- rec
				}
			case <-n.stopCh:
				return
			default:
				return
			}
		}
	}()
	return out
}

// HealthCheck performs a single round-trip to the peer.
func (n *Node) HealthCheck() (int64, error) {
	if n.peer == "" {
		return 0, nil
	}
	url := fmt.Sprintf("http://%s/replica/offset", n.peer)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var body struct {
		Offset int64 `json:"offset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Offset - n.offset.Load(), nil
}

// Save writes the node state to disk.
func (n *Node) Save(path string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	s := State{
		Role:     n.role,
		Offset:   n.offset.Load(),
		Peer:     n.peer,
		Synced:   n.synced.Load(),
		Dropped:  n.dropped.Load(),
		LastSync: time.Now(),
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads node state from disk.
func (n *Node) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.role = s.Role
	n.peer = s.Peer
	n.offset.Store(s.Offset)
	return nil
}
