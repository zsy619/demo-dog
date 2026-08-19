package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// handleSeries returns the catalog of known metric names plus per-metric
// cardinality. This is the Prometheus-standard /api/v1/series endpoint,
// the data source for the explore-metrics feature in Grafana and the
// same shape used by Cortex/Mimir/Thanos.
//
// Query parameters:
//   match[]=<selector>   optional, repeats
//   limit=<int>          optional, default 1000, max 10000
func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	q := r.URL.Query()
	limit := 1000
	if raw := q.Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
			if limit > 10_000 {
				limit = 10_000
			}
		}
	}
	catalog := s.SeriesCatalog().Series()
	filters := q["match[]"]
	if len(filters) == 0 {
		filters = q["match"]
	}
	if len(filters) > 0 {
		keep := map[string]bool{}
		for _, f := range filters {
			if name := parseMatchName(f); name != "" {
				keep[name] = true
			}
		}
		filtered := catalog[:0]
		for _, c := range catalog {
			if keep[c.Name] {
				filtered = append(filtered, c)
			}
		}
		catalog = filtered
	}
	if len(catalog) > limit {
		catalog = catalog[:limit]
	}
	type seriesRow struct {
		Name        string `json:"__name__"`
		FirstSeenMs int64  `json:"first_seen_ms,omitempty"`
		LastSeenMs  int64  `json:"last_seen_ms,omitempty"`
	}
	rows := make([]seriesRow, 0, len(catalog))
	for _, c := range catalog {
		rows = append(rows, seriesRow{
			Name:        c.Name,
			FirstSeenMs: c.FirstSeenMs,
			LastSeenMs:  c.LastSeenMs,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "success",
		"data":   rows,
	})
}

// handleMetadata returns the Prometheus-standard metadata endpoint.
// Grafana dashboards probe this to display type hints next to
// metric names. Without it, standard tooling logs missing-metadata
// warnings on every dashboard load.
func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	limit := 1000
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
			if limit > 10_000 {
				limit = 10_000
			}
		}
	}
	catalog := s.SeriesCatalog().Series()
	if len(catalog) > limit {
		catalog = catalog[:limit]
	}
	type metaEntry struct {
		Type string `json:"type"`
		Help string `json:"help"`
		Unit string `json:"unit,omitempty"`
	}
	data := make(map[string][]metaEntry, len(catalog))
	for _, c := range catalog {
		t, u := guessMetricKind(c.Name)
		data[c.Name] = []metaEntry{{Type: t, Help: "demo-dog metric", Unit: u}}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "success",
		"data":   data,
	})
}

// parseMatchName extracts the metric name from a Prometheus match
// selector. Accepts:
//   metric_name
//   {__name__="metric_name"}
// Returns the empty string when malformed.
func parseMatchName(s string) string {
	if s == "" {
		return ""
	}
	if s[0] != '{' {
		return s
	}
	inner := s[1:]
	if len(inner) > 0 && inner[len(inner)-1] == '}' {
		inner = inner[:len(inner)-1]
	}
	const key = "__name__=\""
	for _, kv := range splitMatch(inner, ',') {
		if len(kv) > len(key)+1 && kv[:len(key)] == key {
			return kv[len(key) : len(kv)-1]
		}
	}
	return ""
}

func splitMatch(s string, sep byte) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// guessMetricKind heuristically infers type/unit from a metric name.
func guessMetricKind(name string) (kind, unit string) {
	switch {
	case hasSuffix(name, ".total") || hasSuffix(name, "_total"):
		return "counter", ""
	case hasSuffix(name, ".duration") || hasSuffix(name, "_duration") ||
		hasSuffix(name, ".seconds") || hasSuffix(name, "_seconds"):
		return "histogram", "seconds"
	case hasSuffix(name, ".bytes") || hasSuffix(name, "_bytes"):
		return "gauge", "bytes"
	case hasSuffix(name, ".count") || hasSuffix(name, "_count"):
		return "gauge", ""
	}
	return "gauge", ""
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}
