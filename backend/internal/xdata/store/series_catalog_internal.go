package store

// series_catalog_internal.go:SeriesCatalog 相关私有辅助函数。

import "sort"

// metricKeyOrder 返回所有 metric name 匹配的 hotMetrics 键(顺序不定)。
func (d *Doris) metricKeyOrder(name string) []string {
	out := make([]string, 0, len(d.hotMetrics))
	for k := range d.hotMetrics {
		if _, n := splitMetricKey(k); n == name {
			out = append(out, k)
		}
	}
	return out
}

// splitMetricKey 将 "svc|name" 拆成 (svc, name)。
//
// 若无 '|',则 svc = 整个 k,name = ""。
func splitMetricKey(k string) (svc, name string) {
	for i := 0; i < len(k); i++ {
		if k[i] == '|' {
			return k[:i], k[i+1:]
		}
	}
	return k, ""
}

// labelsKey 把标签 map 序列化为稳定字符串("k1=v1;k2=v2;...")。
//
// nil 或空 map 返回 ""。
func labelsKey(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]byte, 0, 32)
	for _, k := range keys {
		out = append(out, k...)
		out = append(out, '=')
		out = append(out, m[k]...)
		out = append(out, ';')
	}
	return string(out)
}

// copyLabels 深拷贝一个标签 map(nil 安全)。
func copyLabels(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
