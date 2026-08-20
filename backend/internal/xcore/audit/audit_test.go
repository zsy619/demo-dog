package audit

import (
	"testing"
	"time"
)

func newChain() *Chain {
	return New().WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestAppend_FirstHasEmptyPrev(t *testing.T) {
	c := newChain()
	e := c.Append("a", "create", "alice", "doc1", nil)
	if e.PrevHash != "" {
		t.Fatal("first should have empty prev")
	}
	if e.Seq != 1 {
		t.Fatal("seq")
	}
	if e.Hash == "" {
		t.Fatal("hash")
	}
}

func TestAppend_LinksHashes(t *testing.T) {
	c := newChain()
	e1 := c.Append("a", "create", "alice", "doc1", nil)
	e2 := c.Append("a", "update", "bob", "doc1", nil)
	if e2.PrevHash != e1.Hash {
		t.Fatal("should link to previous")
	}
	if e2.Seq != 2 {
		t.Fatal("seq 2")
	}
}

func TestAppend_TenantIsolation(t *testing.T) {
	c := newChain()
	c.Append("a", "x", "u", "t", nil)
	c.Append("b", "y", "u", "t", nil)
	e := c.Append("a", "z", "u", "t", nil)
	if c.TenantSeq("a") != 2 {
		t.Fatal("a seq")
	}
	if c.TenantSeq("b") != 1 {
		t.Fatal("b seq")
	}
	if e.Seq != 2 {
		t.Fatal("e seq")
	}
}

func TestVerify_Clean(t *testing.T) {
	c := newChain()
	c.Append("a", "x", "u", "t", nil)
	c.Append("a", "y", "u", "t", nil)
	c.Append("a", "z", "u", "t", nil)
	if idx := c.Verify("a"); idx != -1 {
		t.Fatalf("expected clean, got %d", idx)
	}
}

func TestVerify_Tamper(t *testing.T) {
	c := newChain()
	c.Append("a", "x", "u", "t", nil)
	c.Append("a", "y", "u", "t", nil)
	c.Append("a", "z", "u", "t", nil)
	c.mu.Lock()
	c.history["a"][1].Action = "TAMPERED"
	c.mu.Unlock()
	if idx := c.Verify("a"); idx != 1 {
		t.Fatalf("expected tamper at 1, got %d", idx)
	}
}

func TestVerify_Empty(t *testing.T) {
	c := newChain()
	if idx := c.Verify("missing"); idx != -1 {
		t.Fatal("empty should be clean")
	}
}

func TestEvents(t *testing.T) {
	c := newChain()
	c.Append("a", "x", "u", "t", map[string]string{"k": "v"})
	events := c.Events("a")
	if len(events) != 1 {
		t.Fatal("count")
	}
	if events[0].Metadata["k"] != "v" {
		t.Fatal("metadata")
	}
}

func TestEvents_Missing(t *testing.T) {
	c := newChain()
	events := c.Events("missing")
	if len(events) != 0 {
		t.Fatal("expected empty")
	}
}

func TestTenantTail(t *testing.T) {
	c := newChain()
	if c.TenantTail("a") != "" {
		t.Fatal("empty tail")
	}
	e := c.Append("a", "x", "u", "t", nil)
	if c.TenantTail("a") != e.Hash {
		t.Fatal("tail mismatch")
	}
}

func TestVerifyAll(t *testing.T) {
	c := newChain()
	c.Append("a", "x", "u", "t", nil)
	c.Append("b", "y", "u", "t", nil)
	res := c.VerifyAll()
	if len(res) != 0 {
		t.Fatalf("expected empty, got %v", res)
	}
}

func TestHashEvent_Stable(t *testing.T) {
	now := time.Unix(1700000000, 0)
	e := &Event{Tenant: "a", Action: "x", Actor: "u", Target: "t", At: now, PrevHash: "", Seq: 1}
	h1 := hashEvent(e)
	h2 := hashEvent(e)
	if h1 != h2 {
		t.Fatal("hash should be stable")
	}
	if len(h1) != 64 {
		t.Fatal("hex length")
	}
}

func TestSortedKeys(t *testing.T) {
	keys := sortedKeys(map[string]string{"b": "1", "a": "2"})
	if keys[0] != "a" || keys[1] != "b" {
		t.Fatal("sort")
	}
}
