package ratelimit

// snapshot.go：监控快照类型与快照 API。
//
// Snapshot / TokenBucketEntry / LeakyBucketEntry 是 JSON 稳定类型，
// 用于向监控系统 / admin 接口暴露当前限流器状态。

// Snapshot 是 Limiter 的 JSON 稳定视图。
//
// 字段语义：
//   - Shards：当前活跃分片总数（令牌桶 + 漏桶）；
//   - TokenKeys / LeakKeys：分别列出每个 key 的当前令牌数 / 积压量。
type Snapshot struct {
	Shards    int                `json:"shards"`               // 活跃分片总数
	TokenKeys []TokenBucketEntry `json:"token_buckets,omitempty"` // 令牌桶快照
	LeakKeys  []LeakyBucketEntry `json:"leak_buckets,omitempty"`  // 漏桶快照
}

// TokenBucketEntry 是单个令牌桶快照行。
type TokenBucketEntry struct {
	Key    string  `json:"key"`    // 分片 key
	Tokens float64 `json:"tokens"` // 当前令牌数（已按当前时间刷新）
}

// LeakyBucketEntry 是单个漏桶快照行。
type LeakyBucketEntry struct {
	Key   string  `json:"key"`   // 分片 key
	Level float64 `json:"level"` // 当前积压量（已按当前时间刷新）
}

// Snapshot 返回 Limiter 当前状态的快照。
//
// 返回值按当前时间刷新令牌数与积压量；空限流器返回零值。
func (l *Limiter) Snapshot() Snapshot {
	now := l.settings.now()()
	l.mu.Lock()
	defer l.mu.Unlock()
	out := Snapshot{}
	for k, b := range l.tb {
		elapsed := now.Sub(b.lastFill).Seconds()
		tokens := b.tokens
		if elapsed > 0 {
			tokens = b.tokens + elapsed*l.settings.refill()
			if tokens > float64(l.settings.capacity()) {
				tokens = float64(l.settings.capacity())
			}
		}
		out.TokenKeys = append(out.TokenKeys, TokenBucketEntry{Key: k, Tokens: tokens})
	}
	for k, b := range l.lb {
		elapsed := now.Sub(b.lastDec).Seconds()
		level := b.level
		if elapsed > 0 {
			level = b.level - elapsed*l.settings.leak()
			if level < 0 {
				level = 0
			}
		}
		out.LeakKeys = append(out.LeakKeys, LeakyBucketEntry{Key: k, Level: level})
	}
	out.Shards = len(l.tb) + len(l.lb)
	return out
}
