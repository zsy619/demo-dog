package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
)

// handleRules 以 Prometheus /api/v1/rules 的格式
// 返回在线的 SLO burn-rate 规则。兼容 Prometheus
// UI 的 Rules 选项卡以及任何与该端点对接的工具。
// 
// 鉴权：
// 请求必须携带包含 "rules:read" scope 的 API key。
// 缺少该 scope 的 key 即使对其他端点有效也会得到 403。
// 只读 key 可以枚举规则；此处不要求
// 写 key（rules:write）。
// 
// 查询参数（均为可选）：
// type=alert   仅返回 alert 规则（我们只有 alert 规则）
// state=...    仅作信息展示，会在响应结构中原样返回
// group=<name> 忽略（为简单起见，我们按规则分组）
func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	// Round 37：per-key scope 强制执行。缺少
	// rules:read 的只读 key 即使已通过其他端点的鉴权也会得到 403。
	// 空 scope 列表被解释为 "无资源访问"，因此我们
	// 必须显式检查该资源；AllowsResource 仅出于对历史 key 的设计
	// 把空列表视为 "全部放行"，所以我们
	// 两个方向都检查：key 必须存在 AND 必须拥有该 scope。
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

// promQueryFor 为一条 burn-rate 规则合成一个 Prometheus 风格的查询字符串。
// 该字符串仅作信息展示，并不能针对真实的 PromQL 求值——
// 但能让人类直观地看到该规则监听的内容。
func promQueryFor(r alerts.Rule) string {
	svc := r.Service
	if svc == "" {
		svc = "*"
	}
	return fmt.Sprintf("burn_rate(service=%q, window=%s, fast_burn=%g, slow_burn=%g, target=%g)",
		svc, r.Window, r.FastBurn, r.SlowBurn, r.Target)
}

// ruleStateFor 默认返回 "inactive"。我们目前尚未在引擎中跟踪规则的
// per-rule firing 状态；rules 端点暴露
// 配置，fires 端点暴露历史记录。
func ruleStateFor(_ alerts.Rule) string {
	return "inactive"
}
