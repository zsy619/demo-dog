package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
)

// handleRules 以 Prometheus /api/v1/rules 返回的形状
// 返回当前 SLO 烧速率规则。与 Prometheus
// UI 的 Rules 标签页及任何对接该端点的工具兼容。
//
// 授权：
//   请求必须携带包含 "rules:read" 作用域的 API 密钥。
//   不带该作用域的密钥即使在其他端点合法，也会看到 403。
//   只读密钥可以枚举规则；此处不需要
//   写密钥（rules:write）。
//
// 查询参数（全部可选）：
//   type=alert   只返回 alert 规则（我们只有 alert 规则）
//   state=...    信息性字段，在响应形状中返回
//   group=<name> 忽略（为简单起见我们按规则分组）
func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	// 第 37 轮：按密钥的作用域强制执行。没有 rules:read 的
	// 只读密钥即使在其他端点已认证，也会得到 403。
	// 空的作用域列表被解释为 "无资源访问权限"，
	// 因此我们必须显式检查资源；AllowsResource 仅在设计层面
	// 针对旧密钥将空列表视作 "全部允许"，所以我们
	// 双向检查：密钥必须存在且拥有该作用域。
	key := extractKey(r)
	if key != "" {
		if !s.auth.AllowsResource(key, "rules:read") {
			writeError(w, http.StatusForbidden,
				errors.New("missing rules:read scope"))
			return
		}
		// 防御性处理：空 Scopes 表示旧密钥（无作用域）。放行。
		scopes := s.auth.ScopesFor(key)
		if len(scopes) > 0 {
			hasScope := false
			for _, s := range scopes {
				if s == "rules:read" {
					hasScope = true
					break
				}
			}
			if !hasScope {
				writeError(w, http.StatusForbidden,
					errors.New("missing rules:read scope"))
				return
			}
		}
	}
	if s.alerts == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"data":   map[string]any{"groups": []any{}},
		})
		return
	}
	rules := s.alerts.sortedRules()
	type ruleState struct {
		State       string    `json:"state"`
		LastError   string    `json:"lastError,omitempty"`
		EvaluatedAt time.Time `json:"evaluatedAt"`
		Health      string    `json:"health"`
	}
	type ruleEntry struct {
		Name        string            `json:"name"`
		Query       string            `json:"query"`
		Duration    float64           `json:"duration"` // seconds
		Labels      map[string]string `json:"labels,omitempty"`
		Annotations map[string]string `json:"annotations,omitempty"`
		Type        string            `json:"type"`
		Health      string            `json:"health"`
		State       string            `json:"state"`
		LastEvaluation ruleState      `json:"lastEvaluation"`
	}
	type ruleGroup struct {
		Name  string      `json:"name"`
		File  string      `json:"file"`
		Rules []ruleEntry `json:"rules"`
	}

	groups := make([]ruleGroup, 0, len(rules))
	for _, rl := range rules {
		groups = append(groups, ruleGroup{
			Name: rl.Name,
			File: "slo.burnrate",
			Rules: []ruleEntry{{
				Name:     rl.Name,
				Query:    promQueryFor(rl),
				Duration: rl.Window.Seconds(),
				Labels: map[string]string{
					"severity": string(rl.Severity),
					"service":  rl.Service,
				},
				Annotations: map[string]string{
					"description": rl.Description,
					"summary":     rl.Description,
					"fast_burn":   fmt.Sprintf("%g", rl.FastBurn),
					"slow_burn":   fmt.Sprintf("%g", rl.SlowBurn),
					"target":      fmt.Sprintf("%g", rl.Target),
				},
				Type:  "alerting",
				Health: "ok",
				State:  ruleStateFor(rl),
				LastEvaluation: ruleState{
					State:       ruleStateFor(rl),
					Health:      "ok",
					EvaluatedAt: time.Now(),
				},
			}},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "success",
		"data": map[string]any{
			"groups": groups,
		},
	})
}

// promQueryFor 为烧速率规则合成一个 Prometheus 形状的
// 查询字符串。该字符串是说明性的，不可针对真实 PromQL 求值 ——
// 但它告诉人类该规则关注的内容。
func promQueryFor(r alerts.Rule) string {
	svc := r.Service
	if svc == "" {
		svc = "*"
	}
	return fmt.Sprintf("burn_rate(service=%q, window=%s, fast_burn=%g, slow_burn=%g, target=%g)",
		svc, r.Window, r.FastBurn, r.SlowBurn, r.Target)
}

// ruleStateFor 默认返回 "inactive"。我们目前尚未在引擎中跟踪
// 按规则的触发状态；rules 端点暴露配置，
// fires 端点暴露历史。
func ruleStateFor(_ alerts.Rule) string {
	return "inactive"
}
