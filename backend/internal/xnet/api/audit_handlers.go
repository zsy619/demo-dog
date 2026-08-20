package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// handleAudit 返回审计日志中最近的 N 条事件,可选地
// 进行过滤。
// Query 参数:
//   - n:        可选,默认 200,最大 10 000
//   - since:    可选 RFC3339 时间戳
//   - until:    可选 RFC3339 时间戳
//   - method:   精确匹配,例如 POST
//   - path:     子串匹配,例如 "/api/"
//   - tenant:   精确匹配
//   - key:      对 label 的精确匹配
//   - status_min, status_max:   HTTP 状态码范围
//
// 需要 admin 角色(由 Handler() 中的路由门控强制)。
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	n := 200
	if raw := q.Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			n = v
			if n > 10_000 {
				n = 10_000
			}
		}
	}
	filter := AuditFilter{
		Method:   q.Get("method"),
		Path:     q.Get("path"),
		Tenant:   q.Get("tenant"),
		KeyLabel: q.Get("key"),
	}
	if raw := q.Get("status_min"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil { filter.StatusMin = v }
	}
	if raw := q.Get("status_max"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil { filter.StatusMax = v }
	}
	if raw := q.Get("since"); raw != "" {
		if v, err := time.Parse(time.RFC3339, raw); err == nil { filter.Since = v }
	}
	if raw := q.Get("until"); raw != "" {
		if v, err := time.Parse(time.RFC3339, raw); err == nil { filter.Until = v }
	}
	events := s.auditLog.Filter(n, filter)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"count":   len(events),
		"filter":  filter,
		"events":  events,
	})
}

// handleAuditStats 返回缓冲统计信息(capacity / buffered /
// total)。调用开销很低,供仪表板使用。
func (s *Server) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.auditLog.Stats())
}

// handleListKeys 返回已注册的 API keys(不包含原始
// secret —— 只有 label + role)。这使得系统
// 可以从 CI 脚本管理:一次部署可以转储当前的 keys
// 并与其期望状态进行 diff。
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries := s.auth.List()
	type out struct {
		Label string `json:"label"`
		Role  string `json:"role"`
		Key   string `json:"key_prefix"`
	}
	list := make([]out, 0, len(entries))
	for _, e := range entries {
		// 永远不要回显完整的 secret —— 前缀足以让
		// 运维人员识别这是哪一条记录。
		prefix := e.Key
		if len(prefix) > 6 {
			prefix = prefix[:6] + "…"
		}
		list = append(list, out{Label: e.Label, Role: e.Role.String(), Key: prefix})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count": len(list),
		"keys":  list,
	})
}
