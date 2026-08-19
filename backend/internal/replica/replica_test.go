package replica

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrimaryEmit_IncrementsOffset(t *testing.T) {
	n := NewPrimary()
	for i := 0; i < 100; i++ {
		n.Emit(1, []byte("data"))
	}
	if n.offset.Load() != 100 {
		t.Fatalf("offset: %d", n.offset.Load())
	}
}

func TestPrimaryHandler_Offset(t *testing.T) {
	n := NewPrimary()
	ts := httptest.NewServer(n.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/replica/offset")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Offset int64  `json:"offset"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Role != "primary" {
		t.Fatalf("role: %s", body.Role)
	}
	if body.Offset != 0 {
		t.Fatalf("offset: %d", body.Offset)
	}
}

func TestPrimaryHandler_WalStream(t *testing.T) {
	n := NewPrimary()
	ts := httptest.NewServer(n.Handler())
	defer ts.Close()
	for i := 0; i < 5; i++ {
		n.Emit(uint32(i), []byte{byte(i)})
	}
	resp, err := http.Get(ts.URL + "/replica/wal?from=0")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "ndjson") {
		t.Fatalf("content-type: %s", ct)
	}
	dec := json.NewDecoder(resp.Body)
	got := 0
	for dec.More() {
		var rec Record
		if err := dec.Decode(&rec); err != nil {
			t.Fatal(err)
		}
		got++
	}
	if got == 0 {
		t.Fatal("expected at least one record")
	}
}

func TestFollower_Tick_AppliesRecords(t *testing.T) {
	primary := NewPrimary()
	ts := httptest.NewServer(primary.Handler())
	defer ts.Close()
	// Extract host:port
	addr := strings.TrimPrefix(ts.URL, "http://")
	follower := NewFollower(addr)
	follower.Start()
	defer follower.Stop()

	for i := 0; i < 10; i++ {
		primary.Emit(uint32(i), []byte{byte(i)})
	}
	// Wait up to 5s for sync.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if follower.synced.Load() >= 10 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if follower.synced.Load() == 0 {
		t.Fatal("follower received no records")
	}
}

func TestFollower_HealthCheck(t *testing.T) {
	primary := NewPrimary()
	primary.Emit(1, []byte("x"))
	primary.Emit(1, []byte("y"))
	ts := httptest.NewServer(primary.Handler())
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")
	follower := NewFollower(addr)
	lag, err := follower.HealthCheck()
	if err != nil {
		t.Fatal(err)
	}
	if lag != 2 {
		t.Fatalf("lag: %d", lag)
	}
}

func TestSaveLoad(t *testing.T) {
	path := t.TempDir() + "/state.json"
	n := NewPrimary()
	for i := 0; i < 5; i++ {
		n.Emit(1, []byte("x"))
	}
	if err := n.Save(path); err != nil {
		t.Fatal(err)
	}
	restored := NewPrimary()
	if err := restored.Load(path); err != nil {
		t.Fatal(err)
	}
	if restored.offset.Load() != 5 {
		t.Fatalf("restored offset: %d", restored.offset.Load())
	}
	if restored.role != RolePrimary {
		t.Fatalf("restored role: %s", restored.role)
	}
}

func TestFollower_NilPeer(t *testing.T) {
	n := NewFollower("")
	n.Start()
	defer n.Stop()
	// Tick is a no-op with empty peer; should not panic.
	time.Sleep(50 * time.Millisecond)
}

func TestStats_Shape(t *testing.T) {
	n := NewPrimary()
	n.Emit(1, []byte("a"))
	n.Emit(2, []byte("b"))
	s := n.Stats()
	if s.Role != RolePrimary {
		t.Fatalf("role: %s", s.Role)
	}
	if s.Offset != 2 {
		t.Fatalf("offset: %d", s.Offset)
	}
	if s.LastSync.IsZero() {
		t.Fatal("last_sync should be set")
	}
}
