package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
)

// handlePromQL 实现一个轻量的 PromQL-lite 求值器，支持针对
// demo-dog 内存存储的最常见查询。这
// 不是一个完整的 PromQL 引擎——它只是一个子集，足以让 Grafana
// 和 Alertmanager 把该采集器当作可直接替换的 Prometheus
// 来用于仪表盘和基础告警。
// 
// 支持的语法：
// 
// metric_name                        # window 内 metric_name 的全部样本
// metric_name{label="x"}             # 按 label 过滤
// sum by (label) (metric_name)       # 跨 label 维度求和
// avg by (label) (metric_name)       # 平均值
// count by (label) (metric_name)     # 计数
// rate(metric_name[1m])              # window 内的每秒速率
// histogram_quantile(0.95, metric_name)
func (s *Server) handlePromQL(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	q := r.URL.Query().Get("query")
	if q == "" {
		writeError(rw, http.StatusBadRequest, errors.New("missing query"))
		return
	}
	timeStr := r.URL.Query().Get("time")
	end := time.Now()
	if timeStr != "" {
		if v, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
			end = time.Unix(v, 0)
		}
	}
	result, err := evalPromQL(q, end, resolveTenant(r), s.store)
	if err != nil {
		writeError(rw, http.StatusBadRequest, err)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(result)
}

type promqlResult struct {
	Status string     `json:"status"`
	Data   promqlData `json:"data"`
}

type promqlData struct {
	ResultType string         `json:"resultType"`
	Result     []promqlSample `json:"result"`
}

type promqlSample struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
	Values [][]any           `json:"values,omitempty"`
}

func evalPromQL(expr string, end time.Time, tenant string, d *store.Doris) (*promqlResult, error) {
	expr = strings.TrimSpace(expr)
	switch {
	case strings.HasPrefix(expr, "rate("):
		return evalRate(expr, end, tenant, d)
	case strings.HasPrefix(expr, "histogram_quantile("):
		return evalHistogramQuantile(expr, end, tenant, d)
	case strings.HasPrefix(expr, "sum "), strings.HasPrefix(expr, "sum("), strings.HasPrefix(expr, "avg "), strings.HasPrefix(expr, "avg("), strings.HasPrefix(expr, "count "), strings.HasPrefix(expr, "count("):
		return evalAggregator(expr, end, tenant, d)
	default:
		return evalSelector(expr, end, tenant, d)
	}
}

var windowRE = regexp.MustCompile(`\[(\d+)([smhd])\]\)`)

func extractWindow(s string) (time.Duration, string, error) {
	m := windowRE.FindStringSubmatch(s)
	if m == nil {
		return 0, "", errors.New("missing [window] in rate()")
	}
	n, _ := strconv.Atoi(m[1])
	var dur time.Duration
	switch m[2] {
	case "s":
		dur = time.Duration(n) * time.Second
	case "m":
		dur = time.Duration(n) * time.Minute
	case "h":
		dur = time.Duration(n) * time.Hour
	case "d":
		dur = time.Duration(n) * 24 * time.Hour
	default:
		return 0, "", fmt.Errorf("unknown unit %q", m[2])
	}
	inside := s[:strings.LastIndex(s, m[0])]
	inside = strings.TrimSuffix(inside, ")")
	inside = strings.TrimPrefix(inside, "rate(")
	return dur, inside, nil
}

func evalRate(expr string, end time.Time, tenant string, d *store.Doris) (*promqlResult, error) {
	window, inner, err := extractWindow(expr)
	if err != nil {
		return nil, err
	}
	start := end.Add(-window)
	samples := fetchRange(d, inner, tenant, start, end)
	if len(samples) == 0 {
		return &promqlResult{Status: "success", Data: promqlData{ResultType: "vector", Result: []promqlSample{}}}, nil
	}
	byLabels := map[string][]model.MetricPoint{}
	for _, s := range samples {
		key := labelKey(s)
		byLabels[key] = append(byLabels[key], s)
	}
	var out []promqlSample
	for k, pts := range byLabels {
		sort.Slice(pts, func(i, j int) bool { return pts[i].Timestamp.Before(pts[j].Timestamp) })
		first := pts[0].Value
		last := pts[len(pts)-1].Value
		rate := (last - first) / window.Seconds()
		out = append(out, promqlSample{
			Metric: parseLabelKey(k),
			Value:  []any{end.Unix(), strconv.FormatFloat(rate, 'f', 4, 64)},
		})
	}
	sort.Slice(out, func(i, j int) bool { return labelKeyStr(out[i].Metric) < labelKeyStr(out[j].Metric) })
	return &promqlResult{Status: "success", Data: promqlData{ResultType: "vector", Result: out}}, nil
}

func evalAggregator(expr string, end time.Time, tenant string, d *store.Doris) (*promqlResult, error) {
	aggRE := regexp.MustCompile(`^(sum|avg|count)(?:\s+by\s*\(([^)]*)\))?\s*\((.+)\)$`)
	m := aggRE.FindStringSubmatch(expr)
	if m == nil {
		return nil, fmt.Errorf("cannot parse aggregator: %q", expr)
	}
	op := m[1]
	dimList := strings.TrimSpace(m[2])
	inner := strings.TrimSpace(m[3])
	var dims []string
	if dimList != "" {
		for _, x := range strings.Split(dimList, ",") {
			dims = append(dims, strings.TrimSpace(x))
		}
	}
	samples := fetchRange(d, inner, tenant, end.Add(-5*time.Minute), end)
	buckets := map[string][]float64{}
	for _, s := range samples {
		k := bucketKey(s, dims)
		buckets[k] = append(buckets[k], s.Value)
	}
	var out []promqlSample
	for k, vs := range buckets {
		var v float64
		switch op {
		case "sum":
			for _, x := range vs {
				v += x
			}
		case "avg":
			for _, x := range vs {
				v += x
			}
			if len(vs) > 0 {
				v /= float64(len(vs))
			}
		case "count":
			v = float64(len(vs))
		}
		out = append(out, promqlSample{
			Metric: parseBucketKey(k),
			Value:  []any{end.Unix(), strconv.FormatFloat(v, 'f', 4, 64)},
		})
	}
	sort.Slice(out, func(i, j int) bool { return labelKeyStr(out[i].Metric) < labelKeyStr(out[j].Metric) })
	return &promqlResult{Status: "success", Data: promqlData{ResultType: "vector", Result: out}}, nil
}

func evalSelector(expr string, end time.Time, tenant string, d *store.Doris) (*promqlResult, error) {
	name, labels := parseSelector(expr)
	_, metrics, _ := d.Snapshot()
	var out []promqlSample
	buckets := map[string][]model.MetricPoint{}
	for _, m := range metrics {
		if tenant != "" && m.TenantID != tenant {
			continue
		}
		if m.Name != name {
			continue
		}
		if !matchesLabels(m, labels) {
			continue
		}
		k := labelKey(m)
		buckets[k] = append(buckets[k], m)
	}
	for k, pts := range buckets {
		sort.Slice(pts, func(i, j int) bool { return pts[i].Timestamp.Before(pts[j].Timestamp) })
		out = append(out, promqlSample{
			Metric: parseLabelKey(k),
			Value:  []any{end.Unix(), strconv.FormatFloat(pts[len(pts)-1].Value, 'f', 4, 64)},
		})
	}
	sort.Slice(out, func(i, j int) bool { return labelKeyStr(out[i].Metric) < labelKeyStr(out[j].Metric) })
	return &promqlResult{Status: "success", Data: promqlData{ResultType: "vector", Result: out}}, nil
}

func evalHistogramQuantile(expr string, end time.Time, tenant string, d *store.Doris) (*promqlResult, error) {
	hqRE := regexp.MustCompile(`^histogram_quantile\(([0-9.]+),\s*(.+)\)$`)
	m := hqRE.FindStringSubmatch(expr)
	if m == nil {
		return nil, fmt.Errorf("histogram_quantile parse: %q", expr)
	}
	q, _ := strconv.ParseFloat(m[1], 64)
	inner := m[2]
	_, metrics, _ := d.Snapshot()
	var values []float64
	for _, p := range metrics {
		if tenant != "" && p.TenantID != tenant {
			continue
		}
		if p.Name != inner {
			continue
		}
		values = append(values, p.Value)
	}
	if len(values) == 0 {
		return &promqlResult{Status: "success", Data: promqlData{ResultType: "vector", Result: []promqlSample{}}}, nil
	}
	sort.Float64s(values)
	idx := int(float64(len(values)) * q)
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return &promqlResult{
		Status: "success",
		Data: promqlData{
			ResultType: "vector",
			Result: []promqlSample{{
				Metric: map[string]string{"__name__": inner},
				Value:  []any{end.Unix(), strconv.FormatFloat(values[idx], 'f', 4, 64)},
			}},
		},
	}, nil
}

func fetchRange(d *store.Doris, selector, tenant string, start, end time.Time) []model.MetricPoint {
	name, _ := parseSelector(selector)
	_, metrics, _ := d.Snapshot()
	var out []model.MetricPoint
	for _, p := range metrics {
		if tenant != "" && p.TenantID != tenant {
			continue
		}
		if p.Name != name {
			continue
		}
		if p.Timestamp.Before(start) || p.Timestamp.After(end) {
			continue
		}
		out = append(out, p)
	}
	return out
}

var selectorRE = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)(\{[^}]*\})?$`)

func parseSelector(s string) (string, map[string]string) {
	labels := map[string]string{}
	if m := selectorRE.FindStringSubmatch(s); m != nil {
		if m[2] != "" {
			for _, p := range strings.Split(strings.Trim(m[2], "{}"), ",") {
				p = strings.TrimSpace(p)
				if eq := strings.IndexByte(p, '='); eq > 0 {
					key := p[:eq]
					val := strings.Trim(p[eq+1:], "\"")
					labels[key] = val
				}
			}
		}
		return m[1], labels
	}
	return s, labels
}

func matchesLabels(p model.MetricPoint, want map[string]string) bool {
	for k, v := range want {
		switch k {
		case "service":
			if p.Service != v {
				return false
			}
		case "__name__", "name":
			if p.Name != v {
				return false
			}
		}
	}
	return true
}

func labelKey(p model.MetricPoint) string {
	return fmt.Sprintf("service=%s,name=%s", p.Service, p.Name)
}

func bucketKey(p model.MetricPoint, dims []string) string {
	if len(dims) == 0 {
		return labelKey(p)
	}
	parts := make([]string, len(dims))
	for i, d := range dims {
		switch d {
		case "service":
			parts[i] = "service=" + p.Service
		case "name":
			parts[i] = "name=" + p.Name
		default:
			parts[i] = d + "="
		}
	}
	return strings.Join(parts, ",")
}

func parseLabelKey(s string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			out[kv[:eq]] = kv[eq+1:]
		}
	}
	return out
}

func parseBucketKey(s string) map[string]string { return parseLabelKey(s) }

func labelKeyStr(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return strings.Join(parts, ",")
}
