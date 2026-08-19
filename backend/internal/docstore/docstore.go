package docstore

import (
	"errors"
	"sort"
	"sync"
)

// Doc is the stored record.
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

// New creates an empty store.
func New() *Store {
	return &Store{
		docs:    make(map[string]*Doc),
		indexes: make(map[string]map[string]map[string]struct{}),
	}
}

// ErrNotFound is returned when the requested ID is missing.
var ErrNotFound = errors.New("doc not found")

// Put inserts or replaces a document. The doc must have an ID.
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

// Get returns the document by ID.
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

// Delete removes a document.
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

// FindByIndex returns all docs where field == value.
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

// Count returns the number of documents.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs)
}

// ListIDs returns all doc IDs sorted.
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
