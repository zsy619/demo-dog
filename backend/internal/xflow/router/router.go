// Package router HTTP 路由：路径段匹配 + 方法分发。
package router

import (
	"errors"
	"strings"
	"sync"
)

// Handler 是路由处理函数。
type Handler func(w any, r any)

// Method 是一种 HTTP 方法。
type Method string

const (
	GET    Method = "GET"
	POST   Method = "POST"
	PUT    Method = "PUT"
	DELETE Method = "DELETE"
)

// Route 是一条已注册路由。
type Route struct {
	Method  Method
	Pattern string
	Handler Handler
}

// ErrNotFound 在没有匹配的路由时返回。
var ErrNotFound = errors.New("route not found")

// ErrBadPattern 在模式格式错误时返回。
var ErrBadPattern = errors.New("bad pattern")

// Router 是带方法匹配的模板化路由 trie。
type Router struct {
	mu   sync.RWMutex
	root *node
}

type node struct {
	segment  string
	param    string
	children map[string]*node
	handlers map[Method]Handler
	paramCh  *node
}

func newNode(seg string) *node {
	return &node{
		segment:  seg,
		children: make(map[string]*node),
		handlers: make(map[Method]Handler),
	}
}

// New 创建一个空 Router。
func New() *Router {
	return &Router{root: newNode("")}
}

// Register 注册一个 pattern + method + handler。
// Pattern segments are slash-separated. A segment of ":name"
// is a wildcard parameter.
func (r *Router) Register(method Method, pattern string, h Handler) error {
	if h == nil {
		return ErrBadPattern
	}
	if !strings.HasPrefix(pattern, "/") {
		return ErrBadPattern
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	segs := splitSegments(pattern)
	cur := r.root
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			if cur.paramCh == nil {
				cur.paramCh = newNode(s[1:])
				cur.paramCh.param = s[1:]
			}
			cur = cur.paramCh
			continue
		}
		child, ok := cur.children[s]
		if !ok {
			child = newNode(s)
			cur.children[s] = child
			if i == 0 {
				// 未使用；仅做初始化
			}
		}
		cur = child
	}
	cur.handlers[method] = h
	return nil
}

// Match 查找 (method, path) 的处理函数并返回
// extracted parameters.
func (r *Router) Match(method Method, path string) (Handler, map[string]string, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, nil, ErrBadPattern
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	segs := splitSegments(path)
	params := make(map[string]string)
	n, ok := r.walk(r.root, segs, params)
	if !ok {
		return nil, nil, ErrNotFound
	}
	h, ok := n.handlers[method]
	if !ok {
		return nil, nil, ErrNotFound
	}
	return h, params, nil
}

func (r *Router) walk(n *node, segs []string, params map[string]string) (*node, bool) {
	if len(segs) == 0 {
		return n, true
	}
	s := segs[0]
	// 先尝试静态。
	if child, ok := n.children[s]; ok {
		if m, ok := r.walk(child, segs[1:], params); ok {
			return m, true
		}
	}
	// 尝试参数通配符。
	if n.paramCh != nil {
		params[n.paramCh.param] = s
		if m, ok := r.walk(n.paramCh, segs[1:], params); ok {
			return m, true
		}
	}
	return nil, false
}

// Count 返回已注册的路由数。
func (r *Router) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	r.walkCount(r.root, &n)
	return n
}

func (r *Router) walkCount(n *node, count *int) {
	*count += len(n.handlers)
	for _, c := range n.children {
		r.walkCount(c, count)
	}
	if n.paramCh != nil {
		r.walkCount(n.paramCh, count)
	}
}

func splitSegments(p string) []string {
	if p == "/" {
		return []string{}
	}
	p = strings.TrimPrefix(p, "/")
	return strings.Split(p, "/")
}
