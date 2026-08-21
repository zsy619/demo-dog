package xbilling

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

func TestCounter_RecordAndQuery(t *testing.T) {
	c := NewCounter()
	at := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	c.Record("acme", "invocations", 10, at)
	c.Record("acme", "invocations", 5, at)
	c.Record("acme", "bytes_in", 1024, at)
	got, ok := c.Query("acme", "invocations", "2026-03")
	if !ok || got != 15 {
		t.Errorf("want 15 got %d (ok=%v)", got, ok)
	}
	if _, ok := c.Query("acme", "invocations", "2026-04"); ok {
		t.Errorf("cross-period query must not match")
	}
	if _, ok := c.Query("zen", "invocations", "2026-03"); ok {
		t.Errorf("cross-tenant query must not match")
	}
}

func TestCounter_UsageFor_Aggregates(t *testing.T) {
	c := NewCounter()
	jan := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	c.Record("acme", "invocations", 10, jan)
	c.Record("acme", "invocations", 20, mar)
	c.Record("acme", "bytes_in", 1024, jan)
	usages := c.UsageFor("acme")
	if len(usages) != 2 {
		t.Fatalf("want 2 metrics, got %d", len(usages))
	}
	byMetric := map[string]Usage{}
	for _, u := range usages {
		byMetric[u.Metric] = u
	}
	if inv := byMetric["invocations"]; inv.Total != 30 {
		t.Errorf("invocations total want 30, got %d", inv.Total)
	}
	if inv := byMetric["invocations"]; inv.Periods["2026-01"] != 10 || inv.Periods["2026-03"] != 20 {
		t.Errorf("period buckets wrong: %+v", inv.Periods)
	}
	if bytes := byMetric["bytes_in"]; bytes.Total != 1024 {
		t.Errorf("bytes_in total want 1024, got %d", bytes.Total)
	}
}

func TestEncodeCSV(t *testing.T) {
	rows := []PeriodTotal{
		{Tenant: "acme", Metric: "invocations", Period: "2026-03", Value: 42, UpdatedAt: time.Unix(0, 1711000000000000000).UTC()},
		{Tenant: "zen", Metric: "bytes_in", Period: "2026-03", Value: 1024, UpdatedAt: time.Unix(0, 1711000000000000000).UTC()},
	}
	csv := EncodeCSV(rows)
	if !strings.HasPrefix(string(csv), "period,tenant,metric,value,updated_at\n") {
		t.Errorf("missing header: %s", csv)
	}
	if !strings.Contains(string(csv), "2026-03,acme,invocations,42,") {
		t.Errorf("missing acme row: %s", csv)
	}
	if !strings.Contains(string(csv), "2026-03,zen,bytes_in,1024,") {
		t.Errorf("missing zen row: %s", csv)
	}
}

func TestAggregator_Persistence_SurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	a1, err := NewAggregator(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	at := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	a1.Record("acme", "invocations", 100, at)
	a1.Record("acme", "invocations", 200, at)
	a1.Record("acme", "bytes_in", 4096, at)
	a1.Record("zen", "invocations", 50, at)
	_ = kv.Close()

	// 重启,新 KV handle、新 Aggregator
	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()
	a2, err := NewAggregator(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if got, ok := a2.Query("acme", "invocations", "2026-03"); !ok || got != 300 {
		t.Errorf("acme invocations not restored: %d (ok=%v)", got, ok)
	}
	if got, ok := a2.Query("acme", "bytes_in", "2026-03"); !ok || got != 4096 {
		t.Errorf("acme bytes_in not restored: %d (ok=%v)", got, ok)
	}
	if got, ok := a2.Query("zen", "invocations", "2026-03"); !ok || got != 50 {
		t.Errorf("zen invocations not restored: %d (ok=%v)", got, ok)
	}
}

func TestAggregator_Persistence_KeysLandInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer kv.Close()
	ctx := context.Background()
	a, err := NewAggregator(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	at := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	a.Record("acme", "invocations", 1, at)
	keys, err := kv.List(ctx, "metering/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "metering/2026-03/acme/invocations" {
		t.Errorf("want [metering/2026-03/acme/invocations], got %v", keys)
	}
}

func TestAggregator_NilKV_Rejected(t *testing.T) {
	if _, err := NewAggregator(context.Background(), nil); err == nil {
		t.Error("NewAggregator(nil KV) should error")
	}
}

func TestAggregator_JsonShape_StableAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	a, err := NewAggregator(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	at := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	a.Record("acme", "invocations", 42, at)
	// 直接 dump KV value 确认 JSON 字段名稳定
	raw, err := kv.Get(ctx, "metering/2026-03/acme/invocations")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"tenant", "metric", "period", "value", "updated_at"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing field %s in %v", k, m)
		}
	}
	if m["value"].(float64) != 42 {
		t.Errorf("value mismatch: %v", m["value"])
	}
	_ = kv.Close()
}

func TestAggregator_NilInputs_Silent(t *testing.T) {
	a, err := NewAggregator(context.Background(), mustKV(t))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer a.kv.Close()
	// 空 tenant / 空 metric 都不应 panic 也不应写入 KV。
	a.Record("", "x", 1, time.Now())
	a.Record("a", "", 1, time.Now())
	if len(a.All()) != 0 {
		t.Errorf("empty inputs must not record")
	}
}

func mustKV(t *testing.T) xpersistence.KV {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	return kv
}
