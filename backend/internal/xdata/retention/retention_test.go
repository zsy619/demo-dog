package retention

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPolicies(t *testing.T) {
	defs := DefaultPolicies()
	if defs[TierFree].HotTTL != 24*time.Hour {
		t.Fatal("free hot")
	}
	if defs[TierPro].ColdTTL != 30*24*time.Hour {
		t.Fatal("pro cold")
	}
}

func TestManager_Set(t *testing.T) {
	m := NewManager("", nil)
	m.Set("acme", TierPro)
	p, ok := m.Get("acme")
	if !ok {
		t.Fatal("expected policy")
	}
	if p.Tier != TierPro {
		t.Fatalf("tier: %s", p.Tier)
	}
	if p.UpdatedAt.IsZero() {
		t.Fatal("updated at zero")
	}
}

func TestManager_SetUnknownTier(t *testing.T) {
	m := NewManager("", nil)
	m.Set("acme", Tier("unknown"))
	p, _ := m.Get("acme")
	if p.Tier != TierFree {
		t.Fatal("unknown tier should fall back to free")
	}
}

func TestManager_SetPolicy_Validate(t *testing.T) {
	m := NewManager("", nil)
	if err := m.SetPolicy(Policy{}); err == nil {
		t.Fatal("empty tenant should fail")
	}
	if err := m.SetPolicy(Policy{Tenant: "a", HotTTL: -1}); err == nil {
		t.Fatal("negative hot should fail")
	}
	if err := m.SetPolicy(Policy{Tenant: "a", HotTTL: 1 * time.Hour, ColdTTL: 30 * time.Minute}); err == nil {
		t.Fatal("hot>cold should fail")
	}
}

func TestManager_Remove(t *testing.T) {
	m := NewManager("", nil)
	m.Set("a", TierFree)
	m.Remove("a")
	if _, ok := m.Get("a"); ok {
		t.Fatal("expected removed")
	}
}

func TestManager_List(t *testing.T) {
	m := NewManager("", nil)
	m.Set("b", TierFree)
	m.Set("a", TierPro)
	list := m.List()
	if len(list) != 2 {
		t.Fatalf("len: %d", len(list))
	}
	if list[0].Tenant != "a" {
		t.Fatal("not sorted")
	}
}

func TestDecide_Keep(t *testing.T) {
	m := NewManager("", nil)
	m.Set("a", TierFree)
	d := m.Decide("a", time.Hour)
	if d.Action != "keep" {
		t.Fatalf("action: %s", d.Action)
	}
}

func TestDecide_MoveToCold(t *testing.T) {
	m := NewManager("", nil)
	m.Set("a", TierFree)
	d := m.Decide("a", 2*24*time.Hour)
	if d.Action != "move_to_cold" {
		t.Fatalf("action: %s", d.Action)
	}
}

func TestDecide_Drop(t *testing.T) {
	m := NewManager("", nil)
	m.Set("a", TierFree)
	d := m.Decide("a", 30*24*time.Hour)
	if d.Action != "drop" {
		t.Fatalf("action: %s", d.Action)
	}
}

func TestDecide_UnknownTenant(t *testing.T) {
	m := NewManager("", nil)
	d := m.Decide("missing", 30*24*time.Hour)
	if d.Action != "keep" {
		t.Fatal("missing tenant should keep")
	}
}

func TestDecide_Enterprise(t *testing.T) {
	m := NewManager("", nil)
	m.Set("big", TierEnterprise)
	d := m.Decide("big", 100*24*time.Hour)
	if d.Action != "move_to_cold" {
		t.Fatalf("action: %s", d.Action)
	}
}

func TestMoveToCold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cold")
	m := NewManager(dir, nil)
	src := filepath.Join(t.TempDir(), "hot.log")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst, err := m.MoveToCold(src, "acme")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Fatal("data mismatch")
	}
}

func TestMoveToCold_NoColdDir(t *testing.T) {
	m := NewManager("", nil)
	if _, err := m.MoveToCold("/tmp/x", "acme"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSweep(t *testing.T) {
	now := time.Now()
	m := NewManager(filepath.Join(t.TempDir(), "cold"), func() time.Time { return now })
	m.Set("a", TierFree)
	rows := []Row{
		{ID: "1", Tenant: "a", Timestamp: now.Add(-time.Hour)},
		{ID: "2", Tenant: "a", Timestamp: now.Add(-2 * 24 * time.Hour)},
		{ID: "3", Tenant: "a", Timestamp: now.Add(-30 * 24 * time.Hour)},
	}
	var droppedIDs []string
	var movedIDs []string
	res, err := m.Sweep(rows, func(r Row) error {
		droppedIDs = append(droppedIDs, r.ID)
		return nil
	}, func(r Row, dst string) error {
		movedIDs = append(movedIDs, r.ID)
		os.WriteFile(dst, []byte("x"), 0o644)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inspected != 3 {
		t.Fatalf("inspected: %d", res.Inspected)
	}
	if res.Moved != 1 || res.Dropped != 1 {
		t.Fatalf("moved=%d dropped=%d", res.Moved, res.Dropped)
	}
	if len(movedIDs) != 1 || movedIDs[0] != "2" {
		t.Fatalf("moved ids: %v", movedIDs)
	}
	if len(droppedIDs) != 1 || droppedIDs[0] != "3" {
		t.Fatalf("dropped ids: %v", droppedIDs)
	}
}

func TestSweep_DropperError(t *testing.T) {
	now := time.Now()
	m := NewManager("", func() time.Time { return now })
	m.Set("a", TierFree)
	rows := []Row{{ID: "1", Tenant: "a", Timestamp: now.Add(-30 * 24 * time.Hour)}}
	_, err := m.Sweep(rows, func(Row) error { return errors.New("nope") }, func(Row, string) error { return nil })
	if err == nil {
		t.Fatal("expected dropper error")
	}
}

func TestSweep_NegativeAge(t *testing.T) {
	now := time.Now()
	m := NewManager("", func() time.Time { return now })
	m.Set("a", TierFree)
	rows := []Row{{ID: "1", Tenant: "a", Timestamp: now.Add(time.Hour)}}
	res, _ := m.Sweep(rows, func(Row) error { return nil }, func(Row, string) error { return nil })
	if res.Inspected != 1 || res.Dropped != 0 || res.Moved != 0 {
		t.Fatal("future rows should be skipped")
	}
}

func TestReport(t *testing.T) {
	now := time.Now()
	m := NewManager("", func() time.Time { return now })
	m.Set("a", TierFree)
	rows := []Row{
		{Tenant: "a", Timestamp: now.Add(-time.Hour)},
		{Tenant: "a", Timestamp: now.Add(-2 * 24 * time.Hour)},
		{Tenant: "a", Timestamp: now.Add(-30 * 24 * time.Hour)},
	}
	rep := m.Report("a", rows)
	if rep.Keep != 1 || rep.Move != 1 || rep.Drop != 1 {
		t.Fatalf("report: %+v", rep)
	}
}

func TestReport_UnknownTenant(t *testing.T) {
	m := NewManager("", nil)
	rep := m.Report("missing", nil)
	if rep.Tier != TierFree || rep.Keep != 0 {
		t.Fatal("unknown tenant should report free+keep")
	}
}

func TestStats(t *testing.T) {
	m := NewManager("/somewhere", nil)
	m.Set("a", TierPro)
	m.Set("b", TierFree)
	s := m.Stats()
	if s.Tenants != 2 {
		t.Fatal("tenants")
	}
	if s.ColdDir != "/somewhere" {
		t.Fatal("cold dir")
	}
}
