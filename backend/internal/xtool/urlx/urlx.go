// Package urlx 提供对 net/url 的扩展辅助。
package urlx

import (
	"net/url"
	"strings"
)

// Parse 解析 raw，兼容 query 中重复 key 的情况。
func Parse(raw string) (*url.URL, error) {
	return url.Parse(raw)
}

// QueryValues 解析 raw query 字符串为 url.Values；失败返回空。
func QueryValues(raw string) url.Values {
	v, _ := url.ParseQuery(raw)
	return v
}

// Encode 用 URL 编码返回字符串。
func Encode(v url.Values) string {
	return v.Encode()
}

// JoinPath 拼接基础 URL 与多个路径段，自动处理 /。
func JoinPath(base string, parts ...string) string {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	result := base
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p == "" {
			continue
		}
		result += p + "/"
	}
	return strings.TrimRight(result, "/")
}

// Get 从 query 中读取指定 key，缺失返回 def。
func Get(v url.Values, k, def string) string {
	if s := v.Get(k); s != "" {
		return s
	}
	return def
}

// First 读取 key 列表的第一项，缺失返回 def。
func First(v url.Values, k, def string) string {
	if vs := v[k]; len(vs) > 0 && vs[0] != "" {
		return vs[0]
	}
	return def
}

// IsAbsolute 判断 URL 是否为绝对 URL。
func IsAbsolute(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.IsAbs()
}

// HostPort 返回 host 与 port，未指定端口返回 0。
func HostPort(raw string) (string, int, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", 0, false
	}
	host := u.Hostname()
	port := 0
	if p := u.Port(); p != "" {
		for _, c := range p {
			if c < '0' || c > '9' {
				return host, 0, true
			}
			port = port*10 + int(c-'0')
		}
	}
	return host, port, true
}

// Merge 合并多个 Values。
func Merge(vs ...url.Values) url.Values {
	out := url.Values{}
	for _, v := range vs {
		for k, vals := range v {
			out[k] = append(out[k], vals...)
		}
	}
	return out
}
