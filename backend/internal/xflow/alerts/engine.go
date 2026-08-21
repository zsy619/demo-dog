package alerts

// engine.go:Engine 主体 + 规则 CRUD。

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

// Engine 是 burn-rate 告警引擎。
type Engine struct {
	mu       sync.Mutex          // 保护 rules / fires / firing
	rules    []Rule              // 已注册规则
	provider Provider           // 数据提供者
	client   *http.Client        // webhook HTTP 客户端
	fires    []Fire              // 最近触发事件
	firing   map[string]time.Time // 每条规则的最近触发时间
	wg       sync.WaitGroup      // webhook goroutine 计数
}

// NewEngine 构造一个引擎。
func NewEngine(p Provider) *Engine {
	return &Engine{
		provider: p,
		client:   &http.Client{Timeout: 5 * time.Second},
		firing:   make(map[string]time.Time),
	}
}

// SetRules 替换当前规则列表(复制传入)。
func (e *Engine) SetRules(rules []Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append([]Rule(nil), rules...)
}

// Recent 返回最近 n 条 Fire 事件(按时间从旧到新)。
func (e *Engine) Recent(n int) []Fire {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n <= 0 || n > len(e.fires) {
		n = len(e.fires)
	}
	out := make([]Fire, n)
	copy(out, e.fires[len(e.fires)-n:])
	return out
}

// SortedRules 按名称升序返回规则副本。
func (e *Engine) SortedRules() []Rule {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := append([]Rule(nil), e.rules...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// UpsertRule 插入 r 或替换同名规则。
//
// 若有同名规则被替换,返回 (前一个规则, true);否则返回 (零值, false)。
func (e *Engine) UpsertRule(r Rule) (prev Rule, ok bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, existing := range e.rules {
		if existing.Name == r.Name {
			prev = existing
			ok = true
			e.rules[i] = r
			return
		}
	}
	e.rules = append(e.rules, r)
	return
}

// DeleteRule 移除指定规则。
func (e *Engine) DeleteRule(name string) (Rule, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.Name == name {
			out := e.rules[i]
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return out, true
		}
	}
	return Rule{}, false
}

// GetRule 按名称查找规则。
func (e *Engine) GetRule(name string) (Rule, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range e.rules {
		if r.Name == name {
			return r, true
		}
	}
	return Rule{}, false
}
