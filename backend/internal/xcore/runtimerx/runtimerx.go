// Package runtimerx 提供进程运行时的辅助函数：
// Goroutine 数、最大栈深度、GOMAXPROCS 等。
package runtimerx

import (
	"runtime"
	"runtime/debug"
	"sync"
)

// Info 描述进程运行时状态。
type Info struct {
	NumCPU       int    `json:"num_cpu"`
	NumGoroutine int    `json:"num_goroutine"`
	GoVersion    string `json:"go_version"`
	GOMAXPROCS   int    `json:"gomaxprocs"`
	BuildInfo    string `json:"build_info"`
}

// Capture 返回当前运行时信息。
func Capture() Info {
	return Info{
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		GoVersion:    runtime.Version(),
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
		BuildInfo:    buildInfo(),
	}
}

func buildInfo() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return bi.GoVersion + " " + bi.Path
}

// SetMaxProcs 设置 GOMAXPROCS 比例 (0~n)。
func SetMaxProcs(n int) {
	if n < 1 {
		return
	}
	runtime.GOMAXPROCS(n)
}

// NumGoroutine 包装 runtime.NumGoroutine。
func NumGoroutine() int { return runtime.NumGoroutine() }

// Parallel 在 runtime.GOMAXPROCS 范围内并发执行。
func Parallel(items int, fn func(start, end int)) {
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if items < workers {
		workers = items
	}
	if workers < 1 {
		return
	}
	chunk := items / workers
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		start := i * chunk
		end := start + chunk
		if i == workers-1 {
			end = items
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			fn(s, e)
		}(start, end)
	}
	wg.Wait()
}
