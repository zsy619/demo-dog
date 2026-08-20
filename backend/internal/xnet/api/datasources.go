package api

import "sync"

// Datasource 描述采集器可以路由查询的一个逻辑后端。
// 默认注册经典的 Doris 模拟器；
// 注册表可以在不改动 HTTP 处理器的前提下，
// 由未来的后端（ClickHouse、ES、Timescale 等）进行扩展。
type Datasource struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Default       bool     `json:"default,omitempty"`
	URL           string   `json:"url,omitempty"`
	Database      string   `json:"database,omitempty"`
	Tables        []string `json:"tables,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Description   string   `json:"description,omitempty"`
	Version       string   `json:"version,omitempty"`
	PluginVersion string   `json:"plugin_version,omitempty"`
}

// datasourceRegistry 是一个轻量的内存注册表。操作是线程安全的，
// 以便其他包（例如外部的 Doris 驱动插件）
// 可以在启动时进行注册。
type datasourceRegistry struct {
	mu      sync.RWMutex
	sources []Datasource
}

func newDatasourceRegistry() *datasourceRegistry {
	return &datasourceRegistry{
		sources: []Datasource{
			{
				ID:            "doris",
				Name:          "Doris",
				Type:          "olap",
				Default:       true,
				URL:           "http://localhost:9030",
				Database:      "demo_dog",
				Tables:        []string{"__dog_logs", "__dog_metrics", "__dog_traces"},
				Capabilities:  []string{"logs", "metrics", "traces"},
				Description:   "In-memory Doris engine simulating Stream Load ingestion.",
				Version:       "v0.1-demo",
				PluginVersion: "0.1.0",
			},
		},
	}
}

// Add 注册一个新的数据源。调用方提供的 ID 会覆盖
// 任何已存在且 ID 相同的条目
// （因此插件可以在启动时用真实的连接替换内置的 Doris 模拟器）。
func (r *datasourceRegistry) Add(d Datasource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, existing := range r.sources {
		if existing.ID == d.ID {
			r.sources[i] = d
			return
		}
	}
	r.sources = append(r.sources, d)
}

// List 返回已注册数据源的快照。
func (r *datasourceRegistry) List() []Datasource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Datasource, len(r.sources))
	copy(out, r.sources)
	return out
}
