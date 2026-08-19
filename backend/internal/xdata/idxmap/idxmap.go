// Package idxmap 提供一个简化的倒排索引：
// term -> doc id set。
package idxmap

import "sync"

// Index 是一个倒排索引。
type Index struct {
	mu   sync.RWMutex
	term map[string]map[string]struct{}
	docs map[string]struct{}
}

// New 创建一个空 Index。
func New() *Index {
	return &Index{term: make(map[string]map[string]struct{}), docs: make(map[string]struct{})}
}

// Add 写入 doc id 对应的 term。
func (i *Index) Add(docID string, terms ...string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.docs[docID] = struct{}{}
	for _, t := range terms {
		if t == "" {
			continue
		}
		m, ok := i.term[t]
		if !ok {
			m = make(map[string]struct{})
			i.term[t] = m
		}
		m[docID] = struct{}{}
	}
}

// Search 返回匹配任一 term 的文档 id 集合。
func (i *Index) Search(terms ...string) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make(map[string]struct{})
	for _, t := range terms {
		m, ok := i.term[t]
		if !ok {
			continue
		}
		for d := range m {
			out[d] = struct{}{}
		}
	}
	r := make([]string, 0, len(out))
	for d := range out {
		r = append(r, d)
	}
	return r
}

// SearchAll 返回同时匹配所有 term 的文档 id 集合。
func (i *Index) SearchAll(terms ...string) []string {
	if len(terms) == 0 {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	inter := make(map[string]struct{})
	first := true
	for _, t := range terms {
		m, ok := i.term[t]
		if !ok {
			return nil
		}
		if first {
			for d := range m {
				inter[d] = struct{}{}
			}
			first = false
			continue
		}
		for d := range inter {
			if _, ok := m[d]; !ok {
				delete(inter, d)
			}
		}
	}
	r := make([]string, 0, len(inter))
	for d := range inter {
		r = append(r, d)
	}
	return r
}

// DocCount 返回文档数。
func (i *Index) DocCount() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.docs)
}
