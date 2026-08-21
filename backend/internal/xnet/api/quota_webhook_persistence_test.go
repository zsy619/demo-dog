package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xnet/webhook"
	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

// ---- QuotaTracker ----

// TestQuota_Persistence_SetPersists 验证 Set(quota) 后进程
// 重启仍可读到限额配置。
func TestQuota_Persistence_SetPersists(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()

	q1, err := NewQuotaTrackerWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	q1.Set(Quota{
		TenantID:    "acme",
		Window:      time.Hour,
		MaxRequests: 1000,
		MaxBytes:    1 << 20,
	})
	q1.Set(Quota{
		TenantID:    "zen",
		Window:      30 * time.Minute,
		MaxRequests: 500,
		MaxBytes:    1 << 16,
	})
	_ = kv.Close()

	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()

	q2, err := NewQuotaTrackerWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	got, ok := q2.Usage("acme")
	// Usage 没请求过就不存在;走 snapshot 路径:
	// 因为我们只还原了 quotas map,直接看 Snapshot 也只
	// 列出 bucket(为空),所以走一个 Allow 触发 bucket 创建。
	if !ok {
		allowed, _ := q2.Allow("acme", 100)
		if !allowed {
			t.Errorf("acme allow should be true initially")
		}
		got, _ = q2.Usage("acme")
	}
	if got.MaxRequests != 1000 {
		t.Errorf("acme MaxRequests not restored: got %d", got.MaxRequests)
	}
	if got.MaxBytes != 1<<20 {
		t.Errorf("acme MaxBytes not restored: got %d", got.MaxBytes)
	}
	// zen 通过另一条 Allow 触发:
	allowed2, _ := q2.Allow("zen", 50)
	if !allowed2 {
		t.Errorf("zen allow should be true")
	}
	got, _ = q2.Usage("zen")
	if got.MaxRequests != 500 {
		t.Errorf("zen MaxRequests not restored: got %d", got.MaxRequests)
	}
}

// TestQuota_Persistence_RemoveClears 验证 Remove 后 KV
// 中也清掉,重启不再看到。
func TestQuota_Persistence_RemoveClears(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()

	q1, err := NewQuotaTrackerWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	q1.Set(Quota{
		TenantID:    "acme",
		Window:      time.Hour,
		MaxRequests: 100,
	})
	q1.Remove("acme")
	_ = kv.Close()

	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()

	q2, err := NewQuotaTrackerWithKV(ctx, kv2)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	// Allow 一个没有配额的 tenant 应当直接通过(allow, no usage)
	allowed, usage := q2.Allow("acme", 10)
	if !allowed {
		t.Errorf("removed quota must not block: %+v", usage)
	}
	if usage.MaxRequests != 0 {
		t.Errorf("MaxRequests should be 0 after remove: got %d", usage.MaxRequests)
	}
}

// TestQuota_Persistence_NilKV 验证 nil KV 退化为纯内存。
func TestQuota_Persistence_NilKV(t *testing.T) {
	q := NewQuotaTracker()
	q.Set(Quota{
		TenantID:    "acme",
		Window:      time.Hour,
		MaxRequests: 100,
	})
	allowed, usage := q.Allow("acme", 10)
	if !allowed {
		t.Fatalf("nil KV should still work: %+v", usage)
	}
	if usage.MaxRequests != 100 {
		t.Errorf("MaxRequests should be 100: got %d", usage.MaxRequests)
	}
	q.Remove("acme")
	allowed, _ = q.Allow("acme", 10)
	if !allowed {
		t.Errorf("remove with nil KV should work")
	}
}

// ---- WebhookDispatcher ----

// TestWebhook_Persistence_AddPersists 验证 AddSubscriber 后
// 进程重启仍可读到。
func TestWebhook_Persistence_AddPersists(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()

	d1, err := NewWebhookDispatcherWithKV(ctx, kv, 32)
	if err != nil {
		t.Fatalf("new1: %v", err)
	}
	if err := d1.AddSubscriber(&webhook.Subscriber{
		ID:         "slack-prod",
		URL:        "https://hooks.slack.test/abc",
		Secret:     "shhh",
		EventTypes: []string{"alert.fired", "webhook.test"},
		MaxRetries: 3,
		Timeout:    5 * time.Second,
	}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := d1.AddSubscriber(&webhook.Subscriber{
		ID:     "pd-1",
		URL:    "https://events.pd.test/v2",
		Secret: "tok",
	}); err != nil {
		t.Fatalf("add2: %v", err)
	}
	_ = kv.Close()

	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()

	d2, err := NewWebhookDispatcherWithKV(ctx, kv2, 32)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	subs := d2.Dispatcher().Subscribers()
	if len(subs) != 2 {
		t.Fatalf("got %d subscribers after restart, want 2", len(subs))
	}
	var seenSlack, seenPD bool
	for _, s := range subs {
		switch s.ID {
		case "slack-prod":
			seenSlack = true
			if s.URL != "https://hooks.slack.test/abc" {
				t.Errorf("slack URL wrong: %s", s.URL)
			}
			if s.MaxRetries != 3 {
				t.Errorf("slack MaxRetries wrong: %d", s.MaxRetries)
			}
			if len(s.EventTypes) != 2 || s.EventTypes[0] != "alert.fired" {
				t.Errorf("slack EventTypes wrong: %v", s.EventTypes)
			}
		case "pd-1":
			seenPD = true
		}
	}
	if !seenSlack {
		t.Errorf("slack-prod missing after restart")
	}
	if !seenPD {
		t.Errorf("pd-1 missing after restart")
	}
}

// TestWebhook_Persistence_RemoveClears 验证 Remove 后 KV
// 中也清掉。
func TestWebhook_Persistence_RemoveClears(t *testing.T) {
	kv, path := newSubsystemTestKV(t)
	defer kv.Close()
	ctx := context.Background()

	d1, err := NewWebhookDispatcherWithKV(ctx, kv, 16)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := d1.AddSubscriber(&webhook.Subscriber{
		ID:  "slack-x",
		URL: "https://hooks.slack.test/x",
	}); err != nil {
		t.Fatal(err)
	}
	d1.RemoveSubscriber("slack-x")
	_ = kv.Close()

	kv2, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()

	d2, err := NewWebhookDispatcherWithKV(ctx, kv2, 16)
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if subs := d2.Dispatcher().Subscribers(); len(subs) != 0 {
		t.Errorf("removed subscriber still present: %d", len(subs))
	}
}

// TestWebhook_Persistence_NilKV 验证 nil KV 退化为纯内存。
func TestWebhook_Persistence_NilKV(t *testing.T) {
	d := NewWebhookDispatcher()
	if err := d.AddSubscriber(&webhook.Subscriber{
		ID:  "slack",
		URL: "https://hooks.slack.test/y",
	}); err != nil {
		t.Fatal(err)
	}
	if subs := d.Dispatcher().Subscribers(); len(subs) != 1 {
		t.Errorf("nil KV should still work in-memory: %d", len(subs))
	}
	d.RemoveSubscriber("slack")
	if subs := d.Dispatcher().Subscribers(); len(subs) != 0 {
		t.Errorf("remove with nil KV should work: %d", len(subs))
	}
}

// ---- xpersistence.KV 文件落地 ----

// TestQuota_Persistence_KeysLandInFile 验证落盘键名
// 符合规范(quotas/<tenant>)。
func TestQuota_Persistence_KeysLandInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer kv.Close()
	ctx := context.Background()

	q, err := NewQuotaTrackerWithKV(ctx, kv)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	q.Set(Quota{
		TenantID:    "acme",
		Window:      time.Hour,
		MaxRequests: 100,
	})
	keys, err := kv.List(ctx, "quotas/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "quotas/acme" {
		t.Errorf("want [quotas/acme], got %v", keys)
	}
}

// TestWebhook_Persistence_KeysLandInFile 验证 webhook 键名
// 符合规范(webhooks/<id>)。
func TestWebhook_Persistence_KeysLandInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	kv, err := xpersistence.OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer kv.Close()
	ctx := context.Background()

	d, err := NewWebhookDispatcherWithKV(ctx, kv, 8)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := d.AddSubscriber(&webhook.Subscriber{
		ID:  "pd-main",
		URL: "https://events.pd.test/main",
	}); err != nil {
		t.Fatal(err)
	}
	keys, err := kv.List(ctx, "webhooks/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "webhooks/pd-main" {
		t.Errorf("want [webhooks/pd-main], got %v", keys)
	}
}
