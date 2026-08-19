// Package warm 提供缓存预热协调器：
// 启动时并发从一组 Loader 中加载键值，
// 失败时按指数退避重试，最后把成功项写入目标 KV 存储。
package warm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Loader 是一个键加载器。
type Loader func(ctx context.Context, key string) (any, error)

// Sink 是写入目标。
type Sink interface {
	Put(ctx context.Context, key string, value any) error
}

// Job 描述预热任务的一组键。
type Job struct {
	Name  string
	Keys  []string
}

// Config 是预热配置。
type Config struct {
	Concurrency int
	Timeout     time.Duration
	Retry       int
}

// Default 返回默认配置。
func Default() Config {
	return Config{Concurrency: 8, Timeout: 5 * time.Second, Retry: 1}
}

// Stats 是预热结果统计。
type Stats struct {
	Loaded int64 `json:"loaded"`
	Failed int64 `json:"failed"`
	Total  int64 `json:"total"`
}

// Warm 并发执行 job 中的所有键，结果写入 sink。
func Warm(ctx context.Context, cfg Config, ld Loader, sink Sink, jobs []Job) (Stats, error) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var st stats
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Concurrency)
	for _, j := range jobs {
		for _, k := range j.Keys {
			k := k
			atomic.AddInt64(&st.Total, 1)
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				c, c2 := context.WithTimeout(ctx, cfg.Timeout)
				defer c2()
				var err error
				for attempt := 0; attempt <= cfg.Retry; attempt++ {
					var v any
					v, err = ld(c, k)
					if err == nil {
						if e := sink.Put(c, k, v); e != nil {
							err = e
					} else {
						atomic.AddInt64(&st.Loaded, 1)
						return
					}
				}
				if attempt < cfg.Retry {
					time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
				}
			}
				if err != nil {
					atomic.AddInt64(&st.Failed, 1)
				}
			}()
		}
	}
	wg.Wait()
	if st.Failed > 0 && st.Loaded == 0 {
		return Stats(st), errors.New("warm: 全部失败")
	}
	return Stats(st), nil
}

// stats 是内部计数器；导出通过 Stats(st)。
type stats struct {
	Loaded int64
	Failed int64
	Total  int64
}
