package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// handleSeries 返回已知 metric 名称的目录以及每个 metric 的基数。
// 这是 Prometheus 标准的 /api/v1/series 端点，
// 是 Grafana 中 explore-metrics 功能的数据源，也是
// Cortex/Mimir/Thanos 使用的同一格式。
// 
// 查询参数：
// match[]=<selector>   可选，可重复
// limit=<int>          可选，默认 1000，最大 10000
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

// handleMetadata 返回 Prometheus 标准的 metadata 端点。
// Grafana 仪表盘会探测该端点以在
// metric 名称旁显示类型提示。如果缺少该端点，
// 标准工具会在每次加载仪表盘时输出 missing-metadata 警告。
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

// parseMatchName 从 Prometheus match
// selector 中提取 metric 名称。接受：
// metric_name
// {__name__="metric_name"}
// 在格式不合法时返回空字符串。
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

// guessMetricKind 根据指标名称启发式地推断类型/单位。
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
