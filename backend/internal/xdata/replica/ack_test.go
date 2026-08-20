package replica

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAck_BasicGC(t *testing.T) {
	p := NewPrimaryState(100)
	for i := 1; i <= 10; i++ {
		p.Append(Record{Op: 1, Offset: int64(i)})
	}
	p.RegisterFollower("follower-a", "addr:a")
	p.Ack("follower-a", 10)
	offset, _, _, _ := p.Snapshot()
	if offset != 10 {
		t.Fatalf("primary offset: %d", offset)
	}
	if len(p.retained) != 0 {
		t.Fatalf("expected retention to be GCd, got %d", len(p.retained))
	}
}

func TestAck_MinAcrossFollowers(t *testing.T) {
	p := NewPrimaryState(100)
	p.RegisterFollower("a", "a")
	p.RegisterFollower("b", "b")
	for i := 1; i <= 20; i++ {
		p.Append(Record{Offset: int64(i)})
	}
	p.Ack("a", 20)
	if got := p.minAck.Load(); got != 0 {
		t.Fatalf("minAck after only one ack: %d", got)
	}
	p.Ack("b", 15)
	if got := p.minAck.Load(); got != 15 {
		t.Fatalf("minAck after both acks: %d", got)
	}
	// Retained should still hold records 16..20.
	if len(p.retained) != 5 {
		t.Fatalf("retained: %d", len(p.retained))
	}
}

func TestAck_RetentionCap(t *testing.T) {
	p := NewPrimaryState(20) // small cap
	for i := 1; i <= 100; i++ {
		p.Append(Record{Offset: int64(i)})
	}
	if len(p.retained) > 20 {
		t.Fatalf("retained should be capped, got %d", len(p.retained))
	}
	if p.droppedOldest.Load() == 0 {
		t.Fatal("expected some drops")
	}
}

func TestAckHandler_HTTP(t *testing.T) {
	p := NewPrimaryState(100)
	p.Append(Record{Offset: 5})
	p.Append(Record{Offset: 6})
	ts := httptest.NewServer(p.AckHandler())
	defer ts.Close()
	body, _ := json.Marshal(map[string]any{
		"id":     "follower-a",
		"offset": 6,
	})
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var out struct {
		OK        bool  `json:"ok"`
		MinAck    int64 `json:"min_ack"`
		Retained  int   `json:"retained"`
		AckedNow  int64 `json:"acked_now"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.MinAck != 6 || out.Retained != 0 {
		t.Fatalf("response: %+v", out)
	}
}

func TestAckHandler_BadMethod(t *testing.T) {
	p := NewPrimaryState(10)
	ts := httptest.NewServer(p.AckHandler())
	defer ts.Close()
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestAckHandler_MissingID(t *testing.T) {
	p := NewPrimaryState(10)
	ts := httptest.NewServer(p.AckHandler())
	defer ts.Close()
	body, _ := json.Marshal(map[string]any{"offset": 1})
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestAckHandler_LateFollower(t *testing.T) {
	p := NewPrimaryState(100)
	p.RegisterFollower("a", "a")
	p.RegisterFollower("b", "b")
	for i := 1; i <= 50; i++ {
		p.Append(Record{Offset: int64(i)})
	}
	p.Ack("a", 50)
	// b lags. Records 1..50 are retained.
	if len(p.retained) != 50 {
		t.Fatalf("expected 50 retained, got %d", len(p.retained))
	}
	p.Ack("b", 30)
	// Still 19 retained (31..50).
	if len(p.retained) != 20 {
		t.Fatalf("after partial ack: %d", len(p.retained))
	}
	// b can fetch from its ack point.
	got := p.RetainedForFollower(30)
	if len(got) != 20 {
		t.Fatalf("follower fetch: %d", len(got))
	}
}

func TestSnapshot_FollowerList(t *testing.T) {
	p := NewPrimaryState(100)
	p.RegisterFollower("a", "10.0.0.1")
	p.RegisterFollower("b", "10.0.0.2")
	p.Ack("a", 5)
	time.Sleep(2 * time.Millisecond)
	p.Ack("b", 3)
	_, followers, _, _ := p.Snapshot()
	if len(followers) != 2 {
		t.Fatalf("followers: %d", len(followers))
	}
	for _, f := range followers {
		if !f.Connected {
			t.Fatalf("%s should be connected", f.ID)
		}
		if f.LastSeenMs == 0 {
			t.Fatalf("%s should have last_seen", f.ID)
		}
	}
}
