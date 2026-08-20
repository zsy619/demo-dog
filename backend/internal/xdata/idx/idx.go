// Package idx 通用索引抽象：定义索引接口，便于替换实现。
package idx

import (
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Doc 是一个已索引的文档。
type Doc struct {
	ID     string
	Fields map[string]string
}

// Posting 是一个 (docID -> freq) 条目。
type Posting struct {
	DocID string
	Freq  int
}

// Index 是支持 TF 排序的内存倒排索引。
type Index struct {
	mu     sync.RWMutex
	docs   map[string]*Doc
	terms  map[string][]Posting
	stop   map[string]struct{}
}

// New 构造一个空 Index。
func New() *Index {
	return &Index{
		docs:  make(map[string]*Doc),
		terms: make(map[string][]Posting),
	}
}

// WithStopwords 配置停用词。
func (i *Index) WithStopwords(words []string) *Index {
	i.stop = make(map[string]struct{}, len(words))
	for _, w := range words {
		i.stop[w] = struct{}{}
	}
	return i
}

// Add 插入一个文档。
func (i *Index) Add(d *Doc) {
	if d.ID == "" {
		return
	}
	i.mu.Lock()
	i.docs[d.ID] = d
	freqs := make(map[string]int)
	for _, v := range d.Fields {
		for _, tok := range tokenize(v) {
			if i.stop != nil {
				if _, ok := i.stop[tok]; ok {
					continue
				}
			}
			freqs[tok]++
		}
	}
	for term, f := range freqs {
		post := Posting{DocID: d.ID, Freq: f}
		if existing, ok := i.terms[term]; ok {
			existing = append(existing, post)
			i.terms[term] = existing
		} else {
			i.terms[term] = []Posting{post}
		}
	}
	i.mu.Unlock()
}

// Delete 移除一个文档。
func (i *Index) Delete(id string) {
	i.mu.Lock()
	delete(i.docs, id)
	for term, posts := range i.terms {
		out := posts[:0]
		for _, p := range posts {
			if p.DocID != id {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			delete(i.terms, term)
		} else {
			i.terms[term] = out
		}
	}
	i.mu.Unlock()
}

// Search 返回匹配所有查询词的文档，已排序
// by aggregate term frequency descending.
func (i *Index) Search(query string, limit int) []Hit {
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return nil
	}
	i.mu.RLock()
	scores := make(map[string]int)
	for _, tok := range qTokens {
		for _, p := range i.terms[tok] {
			scores[p.DocID] += p.Freq
		}
	}
	hits := make([]Hit, 0, len(scores))
	for id, sc := range scores {
		if d, ok := i.docs[id]; ok {
			hits = append(hits, Hit{DocID: id, Score: sc, Fields: d.Fields})
		}
	}
	i.mu.RUnlock()
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].DocID < hits[j].DocID
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// Hit 是单个搜索结果。
type Hit struct {
	DocID  string            `json:"doc_id"`
	Score  int               `json:"score"`
	Fields map[string]string `json:"fields"`
}

// Size 返回文档数。
func (i *Index) Size() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.docs)
}

// tokenize 转为小写并按 Unicode 空白字符切分。
func tokenize(s string) []string {
	f := func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	}
	return strings.FieldsFunc(strings.ToLower(s), f)
}
