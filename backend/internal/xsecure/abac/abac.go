// Package abac 提供一个属性访问控制（Attribute-Based Access Control）引擎。
// 它接收主体 / 资源 / 环境三元组的属性集合，
// 依次用一组策略判定是否允许。
package abac

import (
	"errors"
	"fmt"
	"sync"
)

// Subject 表示访问主体。
type Subject struct {
	ID   string         `json:"id"`
	Tags map[string]any `json:"tags"`
}

// Resource 表示被访问的资源。
type Resource struct {
	ID   string         `json:"id"`
	Tags map[string]any `json:"tags"`
}

// Environment 携带上下文信息。
type Environment struct {
	Time  string         `json:"time"`
	IP    string         `json:"ip"`
	Tags  map[string]any `json:"tags"`
}

// Request 是 ABAC 请求的输入。
type Request struct {
	Subject     Subject     `json:"subject"`
	Action      string      `json:"action"`
	Resource    Resource    `json:"resource"`
	Environment Environment `json:"environment"`
}

// Decision 是策略裁决结果。
type Decision int

const (
	DecisionDeny Decision = iota
	DecisionAllow
	DecisionNotApplicable
)

// Effect 是策略效果。
type Effect int

const (
	EffectDeny Effect = iota
	EffectAllow
)

// Policy 是一个策略条目。
type Policy struct {
	Name     string
	Actions  []string
	Subjects []string
	Effect   Effect
	Fn       func(req Request) bool
}

// Engine 是策略求值器。
type Engine struct {
	mu       sync.RWMutex
	policies []Policy
}

// New 创建一个 Engine。
func New() *Engine { return &Engine{} }

// Add 注册一个策略。
func (e *Engine) Add(p Policy) {
	e.mu.Lock()
	e.policies = append(e.policies, p)
	e.mu.Unlock()
}

// ErrNoMatch 在没有任何策略命中时返回。
var ErrNoMatch = errors.New("abac: 无匹配策略")

// Check 在所有策略上求值；首个匹配的策略决定结果。
func (e *Engine) Check(req Request) (Decision, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, p := range e.policies {
		if !contains(p.Actions, req.Action) {
			continue
		}
		if !contains(p.Subjects, req.Subject.ID) && !contains(p.Subjects, "*") {
			continue
		}
		if p.Fn != nil && !p.Fn(req) {
			continue
		}
		switch p.Effect {
		case EffectAllow:
			return DecisionAllow, nil
		case EffectDeny:
			return DecisionDeny, nil
		}
	}
	return DecisionDeny, ErrNoMatch
}

// Allowed 是 Check 的便捷封装：返回布尔。
func (e *Engine) Allowed(req Request) bool {
	d, err := e.Check(req)
	if err != nil {
		return false
	}
	return d == DecisionAllow
}

// PolicyCount 返回策略数。
func (e *Engine) PolicyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.policies)
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// String 便于调试。
func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	default:
		return fmt.Sprintf("unknown(%d)", int(d))
	}
}
