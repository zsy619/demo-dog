// Package config 配置加载：从环境变量 / 配置文件 / 默认值按优先级读取。
package config

// 热加载配置。
//
// Collector 持有一份运行时配置结构，运维可以在不重启的前提下更新它。
// Round 57 引入的 Watcher 会每隔 Interval 轮询一次文件，
// 解析其内容（JSON 或 YAML），并在值发生变化时触发回调。
//
// YAML 支持依赖于第三方包 gopkg.in/yaml.v2。
// 为遵循"仅使用标准库"的约束，默认格式为 JSON；
// YAML 支持需由调用方自行接入 YAML 解析器。本文件中的 Config 结构
// 采用 JSON 友好的形态。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// Config 是 collector 使用的运行时配置结构。
// 故意保持精简，便于运维一眼掌握全部配置项。
type Config struct {
	LogLevel    string        `json:"log_level"`
	IngestAddr  string        `json:"ingest_addr"`
	AdminAddr   string        `json:"admin_addr"`
	Workers     int           `json:"workers"`
	DataDir     string        `json:"data_dir"`
	Sampling    float64       `json:"sampling_rate"`
	PeerTimeout time.Duration `json:"peer_timeout"`
}

// Validate 对配置执行基本的合法性检查。
func (c *Config) Validate() error {
	if c.IngestAddr == "" {
		return errors.New("ingest_addr required")
	}
	if c.Workers <= 0 {
		return errors.New("workers must be positive")
	}
	if c.Sampling < 0 || c.Sampling > 1 {
		return errors.New("sampling_rate must be in [0,1]")
	}
	return nil
}

// Default 返回生产环境默认值。
func Default() Config {
	return Config{
		LogLevel:    "info",
		IngestAddr:  ":8080",
		AdminAddr:   ":9100",
		Workers:     8,
		DataDir:     "/var/lib/demo-dog",
		Sampling:    1.0,
		PeerTimeout: 5 * time.Second,
	}
}

// Parse 将 JSON 字节解析为 Config，并对缺失字段填充默认值。
func Parse(data []byte) (Config, error) {
	var c Config
	if len(data) == 0 {
		return Default(), nil
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("parse config: %w", err)
	}
	if c.IngestAddr == "" {
		c.IngestAddr = Default().IngestAddr
	}
	if c.AdminAddr == "" {
		c.AdminAddr = Default().AdminAddr
	}
	if c.Workers == 0 {
		c.Workers = Default().Workers
	}
	if c.DataDir == "" {
		c.DataDir = Default().DataDir
	}
	if c.LogLevel == "" {
		c.LogLevel = Default().LogLevel
	}
	if c.Sampling == 0 {
		c.Sampling = Default().Sampling
	}
	if c.PeerTimeout == 0 {
		c.PeerTimeout = Default().PeerTimeout
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

// Load 从磁盘读取文件并返回解析后的 Config。
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}

// Hash 计算配置的稳定哈希（用于在不进行深度比较的前提下检测重载）。
func (c *Config) Hash() string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Watcher 轮询文件，当哈希与上一次应用的哈希不一致时触发 OnChange。
type Watcher struct {
	mu        sync.Mutex
	path      string
	interval  time.Duration
	onChange  func(Config)
	cur       Config
	curHash   string
	reloads   int
	errors    int
	stopCh    chan struct{}
	runOnce   bool
	applyNew  bool
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

// Run 启动轮询循环。取消 ctx 即可停止。
func (w *Watcher) Run(ctx context.Context) error {
	w.mu.Lock()
	if w.runOnce {
		w.mu.Unlock()
		return errors.New("watcher already running")
	}
	w.runOnce = true
	w.mu.Unlock()
	// 启动时先应用一次，确保 onChange 始终
	// 收到一个非零配置。
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
	Path      string `json:"path"`
	Interval  string `json:"interval"`
	Reloads   int   `json:"reloads"`
	Errors    int   `json:"errors"`
	Hash      string `json:"hash"`
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
