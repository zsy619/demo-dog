// Package breaker 断路器：跟踪失败率，超过阈值时短路以保护下游。
//
// 本包按类型拆分到多个文件：
//   - state.go     断路器状态机与常量
//   - config.go    阈值配置与哨兵错误
//   - outcome.go   内部调用结果记录
//   - stats.go     统计指标与短路 API
//   - breaker.go   断路器主体与所有公共方法
package breaker

// State 表示断路器的状态机。
//
// 三态转换：
//   - StateClosed（关闭）：所有调用正常通过；
//   - StateOpen（开启）：所有调用立即被拒绝；
//   - StateHalfOpen（半开）：允许少量探测调用验证下游是否恢复。
type State int

const (
	// StateClosed 是默认状态，所有调用正常放行。
	StateClosed State = iota
	// StateOpen 表示熔断已触发，所有 Allow() 返回 false。
	StateOpen
	// StateHalfOpen 是冷却期结束后的探测状态，仅允许 HalfOpenCalls 个调用。
	StateHalfOpen
)

// String 返回状态的字符串表示，便于日志与 metrics 输出。
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}
