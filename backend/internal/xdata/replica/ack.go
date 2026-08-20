package replica

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// FollowerState 跟踪一个 follower 的进度。
type FollowerState struct {
	ID         string `json:"id"`
	Addr       string `json:"addr,omitempty"`
	AckOffset  int64  `json:"ack_offset"`
	LastSeenMs int64  `json:"last_seen_ms"`
	Lag        int64  `json:"lag"`
	Connected  bool   `json:"connected"`
}

// PrimaryState 是集群的 at-least-once primary 视图。
type PrimaryState struct {
	mu                  sync.Mutex
	retained            []Record
	retainedStartOffset int64
	followers           map[string]*FollowerState
	minAck              atomic.Int64
	maxRetained         int
	droppedOldest       atomic.Uint64
	receivedAcks        atomic.Uint64
}

// NewPrimaryState 返回 at-least-once 模式的 primary。
func NewPrimaryState(maxRetained int) *PrimaryState {
	if maxRetained <= 0 {
		maxRetained = 100000
	}
	return &PrimaryState{
		followers:   make(map[string]*FollowerState),
		maxRetained: maxRetained,
	}
}

// RegisterFollower 向集群添加一个 follower。
func (p *PrimaryState) RegisterFollower(id, addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.followers[id]; !ok {
		p.followers[id] = &FollowerState{ID: id, Addr: addr}
	}
}

// Append 在 primary 上记录一条新记录。
func (p *PrimaryState) Append(rec Record) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.retained) == 0 {
		p.retainedStartOffset = rec.Offset
	}
	p.retained = append(p.retained, rec)
	if len(p.retained) > p.maxRetained {
		drop := p.maxRetained / 10
		if drop < 1 {
			drop = 1
		}
		p.retained = p.retained[drop:]
		p.retainedStartOffset += int64(drop)
		p.droppedOldest.Add(uint64(drop))
	}
}

// Ack 记录 follower 的进度。
func (p *PrimaryState) Ack(followerID string, offset int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.followers[followerID]
	if !ok {
		f = &FollowerState{ID: followerID}
		p.followers[followerID] = f
	}
	if offset > f.AckOffset {
		f.AckOffset = offset
	}
	f.LastSeenMs = time.Now().UnixMilli()
	f.Connected = true
	min := int64(1<<62 - 1)
	for _, ff := range p.followers {
		if ff.AckOffset < min {
			min = ff.AckOffset
		}
	}
	p.minAck.Store(min)
	p.receivedAcks.Add(1)
	for len(p.retained) > 0 && p.retained[0].Offset <= min {
		p.retained = p.retained[1:]
		p.retainedStartOffset++
	}
}

// Snapshot 返回当前集群状态。
func (p *PrimaryState) Snapshot() (offset int64, followers []FollowerState, dropped, acks uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, f := range p.followers {
		followers = append(followers, *f)
	}
	// 跟踪高水位标记，以便调用方看到最新偏移
	// even after retention has been GC'd.
	if len(p.retained) > 0 {
		offset = p.retained[len(p.retained)-1].Offset
	} else if p.retainedStartOffset > 0 {
		offset = p.retainedStartOffset - 1
	}
	return offset, followers, p.droppedOldest.Load(), p.receivedAcks.Load()
}

// RetainedForFollower 返回 follower 需要的追平记录。
func (p *PrimaryState) RetainedForFollower(fromOffset int64) []Record {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Record, 0, len(p.retained))
	for _, rec := range p.retained {
		if rec.Offset > fromOffset {
			out = append(out, rec)
		}
	}
	return out
}

// AckHandler returns the HTTP handler for POST /replica/ack.
func (p *PrimaryState) AckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID     string `json:"id"`
			Addr   string `json:"addr,omitempty"`
			Offset int64  `json:"offset"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		p.Ack(req.ID, req.Offset)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"min_ack":   p.minAck.Load(),
			"retained":  len(p.retained),
			"acked_now": req.Offset,
		})
	}
}
