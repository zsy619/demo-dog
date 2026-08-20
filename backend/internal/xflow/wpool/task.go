// Package wpool worker 池:可配置缓冲队列的任务调度。
//
// 本包按类型拆分到多个文件:
//   - task.go    Task 类型
//   - pool.go    Pool 主体与所有调度方法
//   - stats.go   Stats 类型与快照
package wpool

import "context"

// Task 是带租户标签的工作单元,用于按租户实现公平调度。
type Task struct {
	Tenant string                 // 租户标识(空值会被替换为 *default)
	Run    func(ctx context.Context) error // 实际执行函数
}
