package breaker

// stats.go：统计指标与短路 API。
//
// Stats 结构是公共快照类型，外部调用方通过 (*Breaker).Stats() 获取。
// ShortCircuit 用于在不调用 Allow() 的前提下记录一次短路事件并返回 ErrOpen。

// Stats 是 Breaker 计数器快照，便于外部监控 / 上报。
//
// 字段语义：
//   - State：当前状态字符串（closed / open / half-open / unknown）；
//   - Accepted：通过 Allow() 的调用总数（succ + failed）；
//   - Rejected：被 Allow() 拒绝的调用数；
//   - Success / Failed：成功 / 失败样本数；
//   - Shorts：主动调用 ShortCircuit() 的次数。
type Stats struct {
	State    string `json:"state"`     // 状态名
	Accepted uint64 `json:"accepted"`  // 允许通过的调用数
	Rejected uint64 `json:"rejected"`  // 被拒绝的调用数
	Success  uint64 `json:"success"`   // 成功数
	Failed   uint64 `json:"failed"`    // 失败数
	Shorts   uint64 `json:"shorts"`    // 短路调用数
}

// Stats 返回 Breaker 当前计数。
//
// 实现细节：先在持锁状态下读取 state，避免拿到不一致的状态字符串；
// 其他计数器使用 atomic.Load()，无需持锁。
func (b *Breaker) Stats() Stats {
	b.mu.Lock()
	state := b.state
	b.mu.Unlock()
	return Stats{
		State:    state.String(),
		Accepted: b.succ.Load() + b.failed.Load(),
		Rejected: b.rejected.Load(),
		Success:  b.succ.Load(),
		Failed:   b.failed.Load(),
		Shorts:   b.shorts.Load(),
	}
}

// ShortCircuit 增加一次短路计数并返回 ErrOpen。
//
// 用途：当业务方想显式触发一次"被拒绝"事件但不调用 Allow() 时使用，
// 比如熔断器关联的请求被上层中间件截获时。
func (b *Breaker) ShortCircuit() error {
	b.shorts.Add(1)
	return ErrOpen
}
