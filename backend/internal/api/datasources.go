package api

import "sync"

// Datasource describes one logical backend that the collector can route
// queries to. The classic Doris simulation is registered by default;
// the registry can be extended by future backends (ClickHouse, ES,
// Timescale, etc.) without touching the HTTP handler.
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

// datasourceRegistry is a tiny in-memory registry. Operations are
// thread-safe so other packages (e.g. an external Doris driver plugin)
// can register at startup.
type datasourceRegistry struct {
	mu       sync.RWMutex
	sources  []Datasource
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

// Add registers a new datasource. Caller-supplied IDs override any
// pre-existing entry with the same ID (so plugins can replace the
// built-in Doris simulator with a real connection at startup).
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

// List returns a snapshot of registered datasources.
func (r *datasourceRegistry) List() []Datasource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Datasource, len(r.sources))
	copy(out, r.sources)
	return out
}
