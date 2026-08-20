// Package grace 提供一个优雅停机协调器。
//
// 用法：
//
//	g := grace.New(30 * time.Second)
//	g.Register(grace.Hook{Name: "http", Fn: srv.Shutdown})
//	g.Register(grace.Hook{Name: "db", Fn: db.Close})
//	if err := g.Run(); err != nil { log.Println(err) }
//
// Shutdown 行为：
//   - 顺序按注册顺序执行所有 Hook
//   - 每个 Hook 在独立的 goroutine 中运行，受总 deadline 约束
//   - 某个 Hook 返回错误：记录并继续下一个（不中断）
//   - 超时：返回 ErrTimeout，剩余 Hook 不再等待（goroutine 仍在运行）
//
// 本包按职责拆分到多个文件：
//   - hook.go     Hook 类型与注册 API
//   - errors.go   哨兵错误
//   - manager.go  Manager 主体与所有公共方法
//   - internal.go 私有辅助函数
package grace

import "context"

// Hook 表示一项关闭动作。
//
// Fn 是必须实现 context.Context 入参的函数，用于支持超时取消；
// Name 是供错误信息与日志标识用的可读字符串。
type Hook struct {
	Name string                      // Hook 名称（错误信息与日志中使用）
	Fn   func(ctx context.Context) error // 关闭动作实现
}
