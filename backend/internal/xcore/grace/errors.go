package grace

import "errors"

// 哨兵错误集合（对外可使用 errors.Is 判断）。

// ErrTimeout 在 Shutdown 总超时时返回。
//
// 此时剩余 Hook 的 goroutine 仍在运行，调用方应继续等待或强制结束进程。
var ErrTimeout = errors.New("grace: 停机超时")

// ErrShutdown 在重复调用 Shutdown 时返回。
//
// 包括：已成功完成后再次调用，或并发调用时除第一个外的其它调用。
var ErrShutdown = errors.New("grace: 重复 Shutdown")
