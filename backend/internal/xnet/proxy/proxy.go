// Package proxy 提供 net/http 代理环境变量解析。
package proxy

import (
	"net/url"
	"os"
)

// FromEnv 返回 HTTP_PROXY / HTTPS_PROXY 等环境变量的解析结果。
func FromEnv() *url.URL {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := os.Getenv(k); v != "" {
			if u, err := url.Parse(v); err == nil {
				return u
			}
		}
	}
	return nil
}

// NoProxy 判断 host 是否匹配 NO_PROXY。
func NoProxy(host string) bool {
	for _, k := range []string{"NO_PROXY", "no_proxy"} {
		if v := os.Getenv(k); v != "" {
			if v == "*" {
				return true
			}
			for _, p := range splitList(v) {
				if match(host, p) {
					return true
				}
			}
		}
	}
	return false
}

// Resolve 返回代理 URL（如果有），nil 表示直连。
func Resolve(reqURL string) *url.URL {
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil
	}
	if NoProxy(u.Hostname()) {
		return nil
	}
	switch u.Scheme {
	case "https":
		if v := os.Getenv("HTTPS_PROXY"); v != "" {
			p, _ := url.Parse(v)
			return p
		}
		if v := os.Getenv("https_proxy"); v != "" {
			p, _ := url.Parse(v)
			return p
		}
	case "http":
		if v := os.Getenv("HTTP_PROXY"); v != "" {
			p, _ := url.Parse(v)
			return p
		}
		if v := os.Getenv("http_proxy"); v != "" {
			p, _ := url.Parse(v)
			return p
		}
	}
	return nil
}

func splitList(s string) []string {
	out := []string{}
	cur := ""
	for _, c := range s {
		if c == ',' || c == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func match(host, pattern string) bool {
	if pattern == host {
		return true
	}
	// 通配 *.example.com
	if len(pattern) >= 2 && pattern[:2] == "*." {
		suf := pattern[1:]
		return len(host) >= len(suf) && host[len(host)-len(suf):] == suf
	}
	return false
}
