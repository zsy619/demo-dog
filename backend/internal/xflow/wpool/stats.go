package wpool

// stats.go:Stats 输出类型与 Pool.Stats() 快照。

// Stats 返回 Pool 的运行计数快照。
type Stats struct {
	Workers  int    `json:"workers"`  // 工作协程数
	Tenants  int    `json:"tenants"`  // 已注册的租户数
	Run      uint64 `json:"run"`      // 累计已执行任务数
	Rejected uint64 `json:"rejected"` // 累计队列满拒绝次数
}

// Stats 返回 Pool 计数器快照。
func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Stats{
		Workers: p.workers, Tenants: len(p.queues),
		Run: p.run.Load(), Rejected: p.reject.Load(),
	}
}
