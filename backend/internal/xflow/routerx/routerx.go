// Package routerx 提供基于 Trie 树的高效 HTTP 路由匹配。
package routerx

import "strings"

// Handler 是匹配后的处理函数。
type Handler func(params map[string]string)

// node 是 trie 节点。
type node struct {
	children map[string]*node
	param    string // 当前参数名（如 :id），空表示静态段
	isStar   bool   // 是否 * 通配
	handler  Handler
	paramSet bool // 防止同层多次设置 param 覆盖
}

// Router 是 trie 路由器。
type Router struct {
	root *node
}

// New 创建 Router。
func New() *Router { return &Router{root: &node{}} }

// Add 注册一个 pattern。
func (r *Router) Add(p string, h Handler) {
	parts := splitPath(p)
	cur := r.root
	for _, seg := range parts {
		if len(seg) > 1 && seg[0] == ':' {
			if cur.children == nil {
				cur.children = make(map[string]*node)
			}
			n := cur.children[":"]
			if n == nil {
				n = &node{param: seg[1:], paramSet: true}
				cur.children[":"] = n
			} else {
				n.param = seg[1:]
			}
			cur = n
			continue
		}
		if seg == "*" {
			if cur.children == nil {
				cur.children = make(map[string]*node)
			}
			n := &node{isStar: true}
			cur.children["*"] = n
			cur = n
			continue
		}
		if cur.children == nil {
			cur.children = make(map[string]*node)
		}
		n := cur.children[seg]
		if n == nil {
			n = &node{}
			cur.children[seg] = n
		}
		cur = n
	}
	cur.handler = h
}

// Match 查找匹配的 handler。
func (r *Router) Match(p string) (Handler, map[string]string) {
	parts := splitPath(p)
	cur := r.root
	params := map[string]string{}
	for _, seg := range parts {
		if cur.isStar {
			break
		}
		if cur.children == nil {
			return nil, nil
		}
		if n, ok := cur.children[seg]; ok {
			cur = n
			continue
		}
		if n, ok := cur.children[":"]; ok {
			params[n.param] = seg
			cur = n
			continue
		}
		if n, ok := cur.children["*"]; ok {
			cur = n
			continue
		}
		return nil, nil
	}
	if cur.handler == nil {
		return nil, nil
	}
	return cur.handler, params
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
