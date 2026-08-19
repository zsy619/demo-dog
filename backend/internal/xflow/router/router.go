// Package router 提供一个简单 HTTP 风格路由匹配器。
package router

import "strings"

// Handler 是匹配后执行的处理函数。
type Handler func(params map[string]string)

// Router 维护一组 pattern -> handler。
type Router struct {
	patterns []pattern
}

type pattern struct {
	parts   []string
	keys    []string
	handler Handler
}

// New 创建 Router。
func New() *Router { return &Router{} }

// Add 注册 pattern，如 /users/:id/posts。
func (r *Router) Add(p string, h Handler) {
	parts := splitPath(p)
	var keys []string
	for _, x := range parts {
		if len(x) > 1 && x[0] == ':' {
			keys = append(keys, x[1:])
		}
	}
	r.patterns = append(r.patterns, pattern{parts: parts, keys: keys, handler: h})
}

// Match 尝试匹配 path，命中时调用 handler。
func (r *Router) Match(path string) (map[string]string, bool) {
	parts := splitPath(path)
	for _, p := range r.patterns {
		if len(p.parts) != len(parts) {
			continue
		}
		params := make(map[string]string)
		ok := true
		for i, seg := range p.parts {
			if len(seg) > 1 && seg[0] == ':' {
				params[seg[1:]] = parts[i]
			} else if seg != parts[i] {
				ok = false
				break
			}
		}
		if ok {
			return params, true
		}
	}
	return nil, false
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
