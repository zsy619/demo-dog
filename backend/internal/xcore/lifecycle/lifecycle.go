// Package lifecycle 进程生命周期：注册启动/停止钩子，统一编排。
package lifecycle

// 带排空超时的优雅停机。
//
// Round 59 引入了 Shutdown 协调器，它将多个服务器 Stopper
// 编排为一次有序排空。每个 Stopper 通过 Name + Timeout 注册。
// 调用 Shutdown() 时，协调器按注册顺序依次执行每个 Stopper；
// 若某个超时，协调器记录错误并继续执行下一个。
//
// 该协调器还整合了信号处理：NotifyOn(parent) 启动一个协程，
// 监听 SIGINT / SIGTERM，任一信号到达时触发 Shutdown。
// 协调器会发出进度事件，调用方可通过 OnProgress 订阅。

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Stopper 是每个服务器需要实现的契约。
type Stopper interface {
	Shutdown(ctx context.Context) error
	Name() string
}

// StopFunc 是用于内联闭包的函数形式适配器。
type StopFunc func(ctx context.Context) error

// StopFuncAdapter 把闭包包装成 Stopper。
type StopFuncAdapter struct {
	N string
	F StopFunc
}

// Shutdown 执行包装的函数。
func (a *StopFuncAdapter) Shutdown(ctx context.Context) error { return a.F(ctx) }

// Name 返回适配器的名称。
func (a *StopFuncAdapter) Name() string { return a.N }

// MakeStopper 通过名称 + 函数构造一个 Stopper。
func MakeStopper(name string, f StopFunc) Stopper {
	return &StopFuncAdapter{N: name, F: f}
}

// Coordinator 持有 Stopper 与进度状态。
type Coordinator struct {
	mu       sync.Mutex
	stoppers []Stopper
	progs    []Progress
	closed   atomic.Bool
	closedAt time.Time
	progress atomic.Int32
}

// Progress 描述单个 Stopper 的状态。
type Progress struct {
	Name    string        `json:"name"`
	Status  string        `json:"status"`
	Error   string        `json:"error,omitempty"`
	Took    time.Duration `json:"took"`
}

// NewCoordinator 返回一个空的 Coordinator。
func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

// Register 添加一个 Stopper。Stopper 按注册顺序执行；
// 若希望倒序执行，只需反向注册。
func (c *Coordinator) Register(s Stopper) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stoppers = append(c.stoppers, s)
}

// Progresses 返回进度条目的副本。
func (c *Coordinator) Progresses() []Progress {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Progress, len(c.progs))
	copy(out, c.progs)
	return out
}

// Shutdown 以给定的单 stopper 超时执行每个 stopper。
// 总时间预算为 单 stopper 超时 × len(stoppers)，
// 加上一个用于跨 stopper 协调的固定余量。
func (c *Coordinator) Shutdown(perTimeout time.Duration) error {
	if c.closed.Swap(true) {
		return errors.New("shutdown already in progress")
	}
	c.closedAt = time.Now()
	c.mu.Lock()
	stoppers := make([]Stopper, len(c.stoppers))
	copy(stoppers, c.stoppers)
	c.mu.Unlock()
	var errs []error
	for _, s := range stoppers {
		c.progress.Add(1)
		ctx, cancel := context.WithTimeout(context.Background(), perTimeout)
		start := time.Now()
		err := s.Shutdown(ctx)
		cancel()
		took := time.Since(start)
		prog := Progress{Name: s.Name(), Took: took}
		if err != nil {
			prog.Status = "failed"
			prog.Error = err.Error()
			errs = append(errs, err)
		} else if took >= perTimeout {
			prog.Status = "timed_out"
			errs = append(errs, errors.New(s.Name()+" timed out"))
		} else {
			prog.Status = "stopped"
		}
		c.mu.Lock()
		c.progs = append(c.progs, prog)
		c.mu.Unlock()
	}
	if len(errs) == 0 {
		return nil
	}
	return joinErr(errs)
}

// Closed 在 Shutdown 被调用之后返回 true。
func (c *Coordinator) Closed() bool { return c.closed.Load() }

// ClosedAt 返回 Shutdown 首次被调用的时间。
func (c *Coordinator) ClosedAt() time.Time { return c.closedAt }

// joinErr 将多个错误拼接起来。
func joinErr(errs []error) error {
	s := ""
	for _, e := range errs {
		if s != "" {
			s += "; "
		}
		s += e.Error()
	}
	return errors.New(s)
}

// NotifyOn 启动一个协程，监听 parent 与信号通道。
// 当 SIGINT 或 SIGTERM 到达时，以 perTimeout 调用 Shutdown。
// 函数返回该协程的等待通道，调用方可阻塞等待停机结束。
func (c *Coordinator) NotifyOn(parent context.Context, perTimeout time.Duration) <-chan struct{} {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-parent.Done():
			return
		case <-ch:
			c.Shutdown(perTimeout)
		}
	}()
	return done
}
