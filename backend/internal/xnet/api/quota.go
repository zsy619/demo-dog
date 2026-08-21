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
	Scope       string        `json:"scope"` // "" = 全局默认,其余按 scope 区分(ingest / query / billing / admin)
	Window      time.Duration `json:"window_ns"`
	MaxRequests int64         `json:"max_requests"`
	MaxBytes    int64         `json:"max_bytes"`
}

// QuotaUsage 是当前活跃窗口中的消费量。
type QuotaUsage struct {
	TenantID    string
	Scope       string
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

// quotaBucketKey 把 (tenant, scope) 收敛到一个 map key。
// scope == "" 落到 "__default__" 子命名空间,行为与
// 之前的全局租户配额保持一致。
func quotaBucketKey(tenant, scope string) string {
	if scope == "" {
		scope = "__default__"
	}
	return tenant + "|" + scope
}

func (q *QuotaTracker) Set(quota Quota) {
	if quota.Window <= 0 {
		quota.Window = time.Hour
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.quotas[quota.TenantID] = quota
	// 仅清空与该租户相关的所有 bucket;其他租户不受影响。
	for k := range q.buckets {
		if strings.HasPrefix(k, quota.TenantID+"|") {
			delete(q.buckets, k)
		}
	}
	_ = q.persistQuota(quota)
}

func (q *QuotaTracker) Remove(tenantID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.quotas, tenantID)
	for k := range q.buckets {
		if strings.HasPrefix(k, tenantID+"|") {
			delete(q.buckets, k)
		}
	}
	q.removeQuota(tenantID)
}

// Allow 与 W1.6 之前的行为等价:scope="" 即默认全局窗口。
func (q *QuotaTracker) Allow(tenantID string, bytes int64) (bool, QuotaUsage) {
	return q.AllowScoped(tenantID, "", bytes)
}

// AllowScoped 是 W1.6 引入的 scope 维度入口。每个 (tenant, scope)
// 拥有独立滑动窗口;同一租户的 ingest / query / billing 配额互不干扰。
// 没有为 (tenant, scope) 显式配置配额时,回落到租户默认配额
// (scope == "");仍找不到则放行。
func (q *QuotaTracker) AllowScoped(tenantID, scope string, bytes int64) (bool, QuotaUsage) {
	if tenantID == "" {
		return true, QuotaUsage{}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	quota, ok := q.quotas[tenantID]
	if !ok {
		return true, QuotaUsage{TenantID: tenantID, Scope: scope}
	}
	effective := quota
	effective.Scope = scope
	key := quotaBucketKey(tenantID, scope)
	now := q.now()
	b, ok := q.buckets[key]
	if !ok || now.Sub(b.windowStart) >= effective.Window {
		b = &quotaBucket{
			quota:       effective,
			windowStart: now,
		}
		q.buckets[key] = b
	}
	if b.limited {
		return false, q.usageLocked(b)
	}
	wouldReq := b.requests + 1
	wouldBytes := b.bytes + bytes
	if effective.MaxRequests > 0 && wouldReq > effective.MaxRequests {
		b.limited = true
		b.limitedAt = now.UnixNano()
		return false, q.usageLocked(b)
	}
	if effective.MaxBytes > 0 && wouldBytes > effective.MaxBytes {
		b.limited = true
		b.limitedAt = now.UnixNano()
		return false, q.usageLocked(b)
	}
	b.requests = wouldReq
	b.bytes = wouldBytes
	return true, q.usageLocked(b)
}

func (q *QuotaTracker) Usage(tenantID string) (QuotaUsage, bool) {
	return q.UsageScoped(tenantID, "")
}

// UsageScoped 返回 (tenant, scope) 的当前窗口用量。
func (q *QuotaTracker) UsageScoped(tenantID, scope string) (QuotaUsage, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	b, ok := q.buckets[quotaBucketKey(tenantID, scope)]
	if !ok {
		return QuotaUsage{TenantID: tenantID, Scope: scope}, false
	}
	return q.usageLocked(b), true
}

func (q *QuotaTracker) usageLocked(b *quotaBucket) QuotaUsage {
	return QuotaUsage{
		TenantID:    b.quota.TenantID,
		Scope:       b.quota.Scope,
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
	for k := range q.buckets {
		if strings.HasPrefix(k, tenantID+"|") {
			delete(q.buckets, k)
		}
	}
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
	for _, b := range q.buckets {
		scope := b.quota.Scope
		if scope == "" {
			scope = "default"
		}
		fmt.Fprintf(w, "dog_tenant_quota_requests{tenant=%q,scope=%q} %d\n", b.quota.TenantID, scope, b.requests)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_bytes Bytes consumed in the current quota window.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_bytes counter\n")
	for _, b := range q.buckets {
		scope := b.quota.Scope
		if scope == "" {
			scope = "default"
		}
		fmt.Fprintf(w, "dog_tenant_quota_bytes{tenant=%q,scope=%q} %d\n", b.quota.TenantID, scope, b.bytes)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_limited Whether the tenant is currently in the limited state.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_limited gauge\n")
	for _, b := range q.buckets {
		scope := b.quota.Scope
		if scope == "" {
			scope = "default"
		}
		v := 0
		if b.limited {
			v = 1
		}
		fmt.Fprintf(w, "dog_tenant_quota_limited{tenant=%q,scope=%q} %d\n", b.quota.TenantID, scope, v)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_max_requests Configured request cap for the current window.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_max_requests gauge\n")
	for tenantID, quota := range q.quotas {
		scope := quota.Scope
		if scope == "" {
			scope = "default"
		}
		fmt.Fprintf(w, "dog_tenant_quota_max_requests{tenant=%q,scope=%q} %d\n", tenantID, scope, quota.MaxRequests)
	}
	fmt.Fprintf(w, "# HELP dog_tenant_quota_max_bytes Configured byte cap for the current window.\n")
	fmt.Fprintf(w, "# TYPE dog_tenant_quota_max_bytes gauge\n")
	for tenantID, quota := range q.quotas {
		scope := quota.Scope
		if scope == "" {
			scope = "default"
		}
		fmt.Fprintf(w, "dog_tenant_quota_max_bytes{tenant=%q,scope=%q} %d\n", tenantID, scope, quota.MaxBytes)
	}
}
