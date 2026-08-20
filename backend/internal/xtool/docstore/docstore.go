// Package docstore 文档存储辅助：基于 key-value 的文档 CRUD。
package docstore

import (
	"errors"
	"sort"
	"sync"
)

// Doc 是存储的记录。
type Doc struct {
	ID     string
	Values map[string]any
}

// Store is an in-memory document store with secondary
// indexes on string-typed fields.
type Store struct {
	mu       sync.RWMutex
	docs     map[string]*Doc
	indexes  map[string]map[string]map[string]struct{} // field -> value -> set of IDs
}

// New 创建一个空的存储。
func New() *Store {
	return &Store{
		docs:    make(map[string]*Doc),
		indexes: make(map[string]map[string]map[string]struct{}),
	}
}

// ErrNotFound 在请求的 ID 缺失时返回。
var ErrNotFound = errors.New("doc not found")

// Put 插入或替换一个文档。该文档必须具有 ID。
func (s *Store) Put(d *Doc) error {
	if d.ID == "" {
		return errors.New("doc ID required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, hasOld := s.docs[d.ID]
	if hasOld {
		for k, v := range existing.Values {
			s.dropFromIndex(k, v, d.ID)
		}
	}
	s.docs[d.ID] = d
	for k, v := range d.Values {
		s.addToIndex(k, v, d.ID)
	}
	return nil
}

// Get 按 ID 返回文档。
func (s *Store) Get(id string) (*Doc, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.docs[id]
	if !ok {
		return nil, false
	}
	cp := copyDoc(d)
	return cp, true
}

// Delete 删除一个文档。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[id]
	if !ok {
		return ErrNotFound
	}
	for k, v := range d.Values {
		s.dropFromIndex(k, v, id)
	}
	delete(s.docs, id)
	return nil
}

// FindByIndex 返回所有 field == value 的文档。
func (s *Store) FindByIndex(field string, value string) []*Doc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx := s.indexes[field]
	if idx == nil {
		return nil
	}
	ids := idx[value]
	out := make([]*Doc, 0, len(ids))
	for id := range ids {
		if d, ok := s.docs[id]; ok {
			out = append(out, copyDoc(d))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Count 返回文档的数量。
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs)
}

// ListIDs 返回所有已排序的文档 ID。
func (s *Store) ListIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.docs))
	for id := range s.docs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (s *Store) addToIndex(field string, value any, id string) {
	str, ok := value.(string)
	if !ok {
		return
	}
	if _, ok := s.indexes[field]; !ok {
		s.indexes[field] = make(map[string]map[string]struct{})
	}
	if _, ok := s.indexes[field][str]; !ok {
		s.indexes[field][str] = make(map[string]struct{})
	}
	s.indexes[field][str][id] = struct{}{}
}

func (s *Store) dropFromIndex(field string, value any, id string) {
	str, ok := value.(string)
	if !ok {
		return
	}
	idx := s.indexes[field]
	if idx == nil {
		return
	}
	if set := idx[str]; set != nil {
		delete(set, id)
		if len(set) == 0 {
			delete(idx, str)
		}
	}
	if len(idx) == 0 {
		delete(s.indexes, field)
	}
}

func copyDoc(d *Doc) *Doc {
	vals := make(map[string]any, len(d.Values))
	for k, v := range d.Values {
		vals[k] = v
	}
	return &Doc{ID: d.ID, Values: vals}
}
