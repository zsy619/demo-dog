package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
)

// Quota 是每个租户的限额配置。
type Quota struct {
	TenantID    string        `json:"tenant"`
	Window      time.Duration `json:"window_ns"`
	MaxRequests int64         `json:"max_requests"`
	MaxBytes    int64         `json:"max_bytes"`
}

// QuotaUsage 是当前活跃窗口中的消费量。
type QuotaUsage struct {
	TenantID    string
	WindowStart time.Time
	WindowEnd   time.Time
	Requests    int64
	Bytes       int64
	MaxRequests int64
	MaxBytes    int64
	Limited     bool
	LimitedAt   int64
}

type quotaBucket struct {
	quota       Quota
	windowStart time.Time
	requests    int64
	bytes       int64
	limited     bool
	limitedAt   int64
}

type QuotaTracker struct {
	mu      sync.RWMutex
	quotas  map[string]Quota
	buckets map[string]*quotaBucket
	now     func() time.Time
	kv      xpersistence.KV // 可选,nil = 纯内存
}

func NewQuotaTracker() *QuotaTracker {
	return &QuotaTracker{
		quotas:  make(map[string]Quota),
		buckets: make(map[string]*quotaBucket),
		now:     time.Now,
	}
}

// NewQuotaTrackerWithKV 创建带持久化的配额跟踪器。
//
// 仅持久化 quotas (租户的限额配置),buckets (当前窗口
// 用量) 是运行时数据,不落盘;进程重启后窗口自然重置。
//
// 存储 key 命名:quotas/<tenant>  →  JSON Quota。
func NewQuotaTrackerWithKV(ctx context.Context, kv xpersistence.KV) (*QuotaTracker, error) {
	if kv == nil {
		return nil, errors.New("quota: kv is nil")
	}
	q := &QuotaTracker{
		quotas:  make(map[string]Quota),
		buckets: make(map[string]*quotaBucket),
		now:     time.Now,
		kv:      kv,
	}
	if err := q.load(ctx); err != nil {
		return nil, fmt.Errorf("quota: load: %w", err)
	}
	return q, nil
}

// SetKV 用于测试或后注入 KV。
func (q *QuotaTracker) SetKV(ctx context.Context, kv xpersistence.KV) error {
	if kv == nil {
		return errors.New("quota: kv is nil")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.quotas) > 0 {
		return errors.New("quota: SetKV on non-empty tracker")
	}
	q.kv = kv
	return q.load(ctx)
}

// load 从 KV 读所有 Quota。
func (q *QuotaTracker) load(ctx context.Context) error {
	keys, err := q.kv.List(ctx, "quotas/")
	if err != nil {
		return err
	}
	for _, k := range keys {
		raw, err := q.kv.Get(ctx, k)
		if err != nil {
			continue
		}
		var quota Quota
		if err := json.Unmarshal(raw, &quota); err != nil {
			continue
		}
		tenant := strings.TrimPrefix(k, "quotas/")
		if tenant == "" {
			continue
		}
		quota.TenantID = tenant
		q.quotas[tenant] = quota
	}
	return nil
}

// persistQuota 把配额写到 KV。
func (q *QuotaTracker) persistQuota(quota Quota) error {
	if q.kv == nil {
		return nil
	}
	raw, err := json.Marshal(quota)
	if err != nil {
		return err
	}
	return q.kv.Set(context.Background(), "quotas/"+quota.TenantID, raw)
}

// removeQuota 从 KV 删一条配额。
func (q *QuotaTracker) removeQuota(tenant string) {
	if q.kv == nil {
		return
	}
	_ = q.kv.Delete(context.Background(), "quotas/"+tenant)
}

func (q *QuotaTracker) Set(quota Quota) {
	if quota.Window <= 0 {
		quota.Window = time.Hour
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.quotas[quota.TenantID] = quota
	delete(q.buckets, quota.TenantID)
	_ = q.persistQuota(quota)
}

func (q *QuotaTracker) Remove(tenantID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.quotas, tenantID)
	delete(q.buckets, tenantID)
	q.removeQuota(tenantID)
}

func (q *QuotaTracker) Allow(tenantID string, bytes int64) (bool, QuotaUsage) {
	q.mu.Lock()
	defer q.mu.Unlock()
	quota, ok := q.quotas[tenantID]
	if !ok {
		return true, QuotaUsage{TenantID: tenantID}
	}
	now := q.now()
	b, ok := q.buckets[tenantID]
	if !ok || now.Sub(b.windowStart) >= quota.Window {
		b = &quotaBucket{
			quota:       quota,
			windowStart: now,
		}
		q.buckets[tenantID] = b
	}
	if b.limited {
		return false, q.usageLocked(b)
	}
	wouldReq := b.requests + 1
	wouldBytes := b.bytes + bytes
	if quota.MaxRequests > 0 && wouldReq > quota.MaxRequests {
		b.limited = true
		b.limitedAt = now.UnixNano()
		return false, q.usageLocked(b)
	}
	if quota.MaxBytes > 0 && wouldBytes > quota.MaxBytes {
		b.limited = true
		b.limitedAt = now.UnixNano()
		return false, q.usageLocked(b)
	}
	b.requests = wouldReq
	b.bytes = wouldBytes
	return true, q.usageLocked(b)
}

func (q *QuotaTracker) Usage(tenantID string) (QuotaUsage, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	b, ok := q.buckets[tenantID]
	if !ok {
		return QuotaUsage{TenantID: tenantID}, false
	}
	return q.usageLocked(b), true
}

func (q *QuotaTracker) usageLocked(b *quotaBucket) QuotaUsage {
	return QuotaUsage{
		TenantID:    b.quota.TenantID,
		WindowStart: b.windowStart,
		WindowEnd:   b.windowStart.Add(b.quota.Window),
		Requests:    b.requests,
		Bytes:       b.bytes,
		MaxRequests: b.quota.MaxRequests,
		MaxBytes:    b.quota.MaxBytes,
		Limited:     b.limited,
		LimitedAt:   b.limitedAt,
	}
}

func (q *QuotaTracker) Reset(tenantID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.buckets, tenantID)
}

func (q *QuotaTracker) Snapshot() []QuotaUsage {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]QuotaUsage, 0, len(q.buckets))
	for _, b := range q.buckets {
		out = append(out, q.usageLocked(b))
	}
	return out
}

func (q *QuotaTracker) WritePrometheus(w io.Writer) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	fmt.Fprintf(w, "# HELP dog_tenant_quota_requests Requests consumed in the current quota window.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_requests counter\n")
	for tenantID, b := range q.buckets {
		fmt.Fprintf(w, "dog_tenant_quota_requests{tenant=%q} %d\n", tenantID, b.requests)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_bytes Bytes consumed in the current quota window.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_bytes counter\n")
	for tenantID, b := range q.buckets {
		fmt.Fprintf(w, "dog_tenant_quota_bytes{tenant=%q} %d\n", tenantID, b.bytes)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_limited Whether the tenant is currently in the limited state.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_limited gauge\n")
	for tenantID, b := range q.buckets {
		v := 0
		if b.limited {
			v = 1
		}
		fmt.Fprintf(w, "dog_tenant_quota_limited{tenant=%q} %d\n", tenantID, v)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_max_requests Configured request cap for the current window.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_max_requests gauge\n")
	for tenantID, quota := range q.quotas {
		fmt.Fprintf(w, "dog_tenant_quota_max_requests{tenant=%q} %d\n", tenantID, quota.MaxRequests)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_max_bytes Configured byte cap for the current window.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_max_bytes gauge\n")
	for tenantID, quota := range q.quotas {
		fmt.Fprintf(w, "dog_tenant_quota_max_bytes{tenant=%q} %d\n", tenantID, quota.MaxBytes)
	}
}
