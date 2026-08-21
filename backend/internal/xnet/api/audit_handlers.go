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
	// 同时兼容 ?n 与前端默认的 ?limit。
	if raw := q.Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			n = v
			if n > 10_000 {
				n = 10_000
			}
		}
	}
	events := s.auditLog.Filter(n, filter)
	// 将内部 AuditEvent 投影成前端表格的紧凑形态。
	out := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		out = append(out, auditEntryView(ev))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":   len(out),
		"entries": out,
	})
}

// auditEntryView 把 AuditEvent 投影为前端审计表所需视图。
//
// 字段:
//   - ts      RFC3339Nano 时间戳
//   - actor   调用方(优先 KeyLabel,其次 Role)
//   - action  HTTP 方法 + 路径,如 "POST /api/ingest/otlp"
//   - target  与 action 相同的路径,便于单独显示
//   - ip      客户端 RemoteIP
//   - ok      true = status 2xx,否则 false
//   - error   失败时为 "status=<code>"
func auditEntryView(ev AuditEvent) map[string]any {
	ok := ev.Status >= 200 && ev.Status < 400
	actor := ev.KeyLabel
	if actor == "" {
		actor = ev.Role
	}
	if actor == "" {
		actor = "anonymous"
	}
	row := map[string]any{
		"ts":     ev.Timestamp.Format(time.RFC3339Nano),
		"actor":  actor,
		"action": ev.Method + " " + ev.Path,
		"target": ev.Path,
		"ip":     ev.RemoteIP,
		"ok":     ok,
	}
	if !ok {
		row["error"] = "status=" + strconv.Itoa(ev.Status)
	}
	return row
}

// handleAuditStats 返回前端仪表盘所需的聚合统计。
//
// 仅遍历当前缓冲,N 由 auditLog.cap 限制,调用开销为 O(N)。
func (s *Server) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	total, ok, failed, byAction := 0, 0, 0, map[string]int{}
	s.auditLog.mu.RLock()
	for _, ev := range s.auditLog.events {
		total++
		if ev.Status >= 200 && ev.Status < 400 {
			ok++
		} else {
			failed++
		}
		action := ev.Method + " " + ev.Path
		byAction[action]++
	}
	s.auditLog.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"total":     total,
		"ok":        ok,
		"failed":    failed,
		"by_action": byAction,
	})
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
