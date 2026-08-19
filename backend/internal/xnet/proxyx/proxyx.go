// Package proxyx 提供代理 URL 解析与构建辅助。
package proxyx

import (
	"errors"
	"strings"
)

// Proxy 表示一个代理配置。
type Proxy struct {
	Scheme string
	Host   string
	Port   int
	User   string
	Pass   string
}

// Parse 解析形如 http://user:pass@host:port 的代理 URL。
func Parse(raw string) (Proxy, error) {
	if raw == "" {
		return Proxy{}, errors.New("proxyx: 空 url")
	}
	p := Proxy{}
	// scheme
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return Proxy{}, errors.New("proxyx: 缺 scheme")
	}
	p.Scheme = raw[:schemeEnd]
	rest := raw[schemeEnd+3:]
	// user:pass@
	if at := strings.Index(rest, "@"); at >= 0 {
		up := rest[:at]
		if c := strings.Index(up, ":"); c >= 0 {
			p.User = up[:c]
			p.Pass = up[c+1:]
		} else {
			p.User = up
		}
		rest = rest[at+1:]
	}
	// host:port
	if h := strings.Index(rest, ":"); h >= 0 {
		p.Host = rest[:h]
		rest = rest[h+1:]
		// 简单解析端口
		port := 0
		for _, ch := range rest {
			if ch < '0' || ch > '9' {
				break
			}
			port = port*10 + int(ch-'0')
		}
		p.Port = port
	} else {
		p.Host = rest
	}
	if p.Host == "" {
		return Proxy{}, errors.New("proxyx: 缺 host")
	}
	return p, nil
}

// String 序列化回 URL。
func (p Proxy) String() string {
	var b strings.Builder
	if p.Scheme != "" {
		b.WriteString(p.Scheme)
		b.WriteString("://")
	}
	if p.User != "" {
		b.WriteString(p.User)
		if p.Pass != "" {
			b.WriteString(":")
			b.WriteString(p.Pass)
		}
		b.WriteString("@")
	}
	b.WriteString(p.Host)
	if p.Port > 0 {
		b.WriteString(":")
		b.WriteString(itoa(p.Port))
	}
	return b.String()
}

// IsZero 判断是否为空。
func (p Proxy) IsZero() bool { return p.Host == "" && p.Scheme == "" }

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
