package config

// watcher.go:Watcher 轮询文件并热加载配置。
//
// Round 57 引入:每隔 Interval 轮询一次文件,解析其内容(默认 JSON),
// 并在哈希发生变化时触发 OnChange 回调。
//
// YAML 支持依赖第三方包 gopkg.in/yaml.v2;
// 为遵循"仅使用标准库"约束,默认格式为 JSON,YAML 支持由调用方接入。

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Watcher 轮询文件,当哈希与上一次应用的哈希不一致时触发 OnChange。
type Watcher struct {
	mu       sync.Mutex     // 保护 cur / curHash / 计数器
	path     string         // 配置文件路径
	interval time.Duration  // 轮询间隔
	onChange func(Config)   // 配置变更回调
	cur      Config         // 当前配置
	curHash  string         // 当前配置哈希
	reloads  int            // 重载次数
	errors   int            // 错误次数
	stopCh   chan struct{}  // 停止信号
	runOnce  bool           // 是否已运行
}

// NewWatcher 返回一个每隔 interval 轮询一次 path 的 Watcher。
func NewWatcher(path string, interval time.Duration, onChange func(Config)) *Watcher {
	return &Watcher{
		path:     path,
		interval: interval,
		onChange: onChange,
		stopCh:   make(chan struct{}),
	}
}

// Run 启动轮询循环;取消 ctx 即可停止。
func (w *Watcher) Run(ctx context.Context) error {
	w.mu.Lock()
	if w.runOnce {
		w.mu.Unlock()
		return errors.New("watcher already running")
	}
	w.runOnce = true
	w.mu.Unlock()
	// 启动时先应用一次,确保 onChange 始终收到一个非零配置。
	cfg, err := Load(w.path)
	if err != nil {
		w.errors++
		return err
	}
	w.apply(cfg)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopCh:
			return nil
		case <-ticker.C:
			cfg, err := Load(w.path)
			if err != nil {
				w.errors++
				continue
			}
			w.apply(cfg)
		}
	}
}

// Stop 通知 Run 返回。
func (w *Watcher) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

// Current 返回最近一次应用的配置快照。
func (w *Watcher) Current() Config {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cur
}

// Stats 表示重载与错误计数器。
type Stats struct {
	Path     string `json:"path"`     // 配置文件路径
	Interval string `json:"interval"` // 轮询间隔
	Reloads  int    `json:"reloads"`  // 重载次数
	Errors   int    `json:"errors"`   // 错误次数
	Hash     string `json:"hash"`     // 当前配置哈希
}

// Stats 返回 Watcher 的计数器。
func (w *Watcher) Stats() Stats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return Stats{
		Path:     w.path,
		Interval: w.interval.String(),
		Reloads:  w.reloads,
		Errors:   w.errors,
		Hash:     w.curHash,
	}
}

// apply 比较 cfg 与当前哈希,变更则触发 onChange 并更新 cur/reloads。
func (w *Watcher) apply(cfg Config) {
	w.mu.Lock()
	newHash := cfg.Hash()
	changed := newHash != w.curHash
	w.mu.Unlock()
	if changed {
		if w.onChange != nil {
			w.onChange(cfg)
		}
		w.mu.Lock()
		w.cur = cfg
		w.curHash = newHash
		w.reloads++
		w.mu.Unlock()
	}
}
