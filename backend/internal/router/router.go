package router

import (
	"errors"
	"strings"
	"sync"
)

// Handler is a route handler function.
type Handler func(w any, r any)

// Method is an HTTP method.
type Method string

const (
	GET    Method = "GET"
	POST   Method = "POST"
	PUT    Method = "PUT"
	DELETE Method = "DELETE"
)

// Route is a registered route.
type Route struct {
	Method  Method
	Pattern string
	Handler Handler
}

// ErrNotFound is returned when no route matches.
var ErrNotFound = errors.New("route not found")

// ErrBadPattern is returned when a pattern is malformed.
var ErrBadPattern = errors.New("bad pattern")

// Router is a templated routing trie with method matching.
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

// New creates an empty Router.
func New() *Router {
	return &Router{root: newNode("")}
}

// Register registers a pattern + method + handler.
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
				// unused; just initialize
			}
		}
		cur = child
	}
	cur.handlers[method] = h
	return nil
}

// Match finds the handler for (method, path) and returns
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
	// Try static first.
	if child, ok := n.children[s]; ok {
		if m, ok := r.walk(child, segs[1:], params); ok {
			return m, true
		}
	}
	// Try param wildcard.
	if n.paramCh != nil {
		params[n.paramCh.param] = s
		if m, ok := r.walk(n.paramCh, segs[1:], params); ok {
			return m, true
		}
	}
	return nil, false
}

// Count returns the number of registered routes.
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
