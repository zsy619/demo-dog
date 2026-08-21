// Package config 配置加载:从环境变量 / 配置文件 / 默认值按优先级读取。
//
// 文件职责拆分:
//   - config.go   Config + Validate/Default/Parse/Load/Hash
//   - watcher.go  Watcher 轮询文件,热加载
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Config 是 collector 使用的运行时配置结构。
//
// 故意保持精简,便于运维一眼掌握全部配置项。
type Config struct {
	LogLevel    string        `json:"log_level"`     // 日志级别
	IngestAddr  string        `json:"ingest_addr"`   // 接入地址
	AdminAddr   string        `json:"admin_addr"`    // 管理地址
	Workers     int           `json:"workers"`        // 工作协程数
	DataDir     string        `json:"data_dir"`       // 数据目录
	Sampling    float64       `json:"sampling_rate"`  // 采样率
	PeerTimeout time.Duration `json:"peer_timeout"`   // 对端超时
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

// Parse 将 JSON 字节解析为 Config,并对缺失字段填充默认值。
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

// Hash 计算配置的稳定哈希,用于在不进行深度比较的前提下检测重载。
func (c *Config) Hash() string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
