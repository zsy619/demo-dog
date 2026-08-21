// Package health 健康检查:探测外部依赖并汇总健康状态。
//
// 本包按类型拆分到多个文件:
//   - check.go      Check 类型 + 三种工厂(HTTP / TCP / Worker)
//   - aggregator.go Aggregator 主体与所有调度方法
//   - snapshot.go   Snapshot 输出类型
//   - internal.go   私有辅助
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Check 表示一个具名健康探测。
type Check struct {
	Name      string                  // 检查名(用于去重/聚合)
	Critical  bool                    // 是否为关键检查(关键失败则整组不健康)
	Probe     func(ctx context.Context) error // 探测回调
	Threshold time.Duration           // 单次探测超时
	Status    string                  // "ok" / "failed"
	Error     string                  // 失败时的错误描述
	Took      time.Duration           // 最近一次探测耗时
	At        time.Time               // 最近一次探测时间
}

// HTTPCheck 构造一个会访问指定 URL 的 Check。
//
// 5xx 视为失败;其他状态码视为成功。
func HTTPCheck(name, url string, critical bool) *Check {
	return &Check{
		Name: name, Critical: critical,
		Probe: func(ctx context.Context) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 500 {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			return nil
		},
	}
}

// TCPCheck 构造一个会建立 TCP 连接的 Check。
//
// 仅做 Dial 验证,不发送应用层数据。
func TCPCheck(name, addr string, critical bool) *Check {
	return &Check{
		Name: name, Critical: critical,
		Probe: func(ctx context.Context) error {
			d := net.Dialer{Timeout: 2 * time.Second}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return err
			}
			conn.Close()
			return nil
		},
	}
}

// WorkerCheck 上报具名 Worker 池的健康状态。
//
// 当 active <= max 时 probe 返回 nil。
func WorkerCheck(name string, active, max int, critical bool) *Check {
	return &Check{
		Name: name, Critical: critical,
		Probe: func(ctx context.Context) error {
			if active > max {
				return fmt.Errorf("active %d > max %d", active, max)
			}
			return nil
		},
	}
}
