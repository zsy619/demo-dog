// Package config 配置加载：从环境变量 / 配置文件 / 默认值按优先级读取。
package config

// Hot-reload configuration.
//
// The collector holds a runtime config struct that the
// operator can update without restarting. Round 57 introduces
// the Watcher that polls a file every Interval, parses it
// (JSON or YAML), and runs a callback when the value changes.
//
// YAML support requires gopkg.in/yaml.v2 which is a third
// party dependency. To honour the stdlib-only constraint the
// default format is JSON; YAML support is conditional on the
// caller wiring a YAML parser. The Config struct here is the
// JSON-friendly shape.

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

// Config is the runtime configuration shape used by the
// collector. It is intentionally small so the operator can
// understand the whole surface at a glance.
type Config struct {
	LogLevel    string        `json:"log_level"`
	IngestAddr  string        `json:"ingest_addr"`
	AdminAddr   string        `json:"admin_addr"`
	Workers     int           `json:"workers"`
	DataDir     string        `json:"data_dir"`
	Sampling    float64       `json:"sampling_rate"`
	PeerTimeout time.Duration `json:"peer_timeout"`
}

// Validate runs basic checks on the config.
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

// Default returns the production defaults.
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

// Parse reads JSON bytes into a Config, applying defaults
// for missing fields.
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

// Load reads a file from disk and returns a parsed Config.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}

// Hash computes a stable hash of the config (used to detect
// reload without doing deep equality).
func (c *Config) Hash() string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Watcher polls a file and fires OnChange when its hash
// differs from the last applied hash.
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

// NewWatcher returns a watcher that polls path every interval.
func NewWatcher(path string, interval time.Duration, onChange func(Config)) *Watcher {
	return &Watcher{
		path:     path,
		interval: interval,
		onChange: onChange,
		stopCh:   make(chan struct{}),
	}
}

// Run starts the polling loop. Cancelling ctx stops it.
func (w *Watcher) Run(ctx context.Context) error {
	w.mu.Lock()
	if w.runOnce {
		w.mu.Unlock()
		return errors.New("watcher already running")
	}
	w.runOnce = true
	w.mu.Unlock()
	// Apply once at start so onChange always gets called
	// with a non-zero config.
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

// Stop signals Run to return.
func (w *Watcher) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

// Current returns the most recently applied config snapshot.
func (w *Watcher) Current() Config {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cur
}

// Stats returns reload + error counters.
type Stats struct {
	Path      string `json:"path"`
	Interval  string `json:"interval"`
	Reloads   int   `json:"reloads"`
	Errors    int   `json:"errors"`
	Hash      string `json:"hash"`
}

// Stats returns the watcher counters.
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
