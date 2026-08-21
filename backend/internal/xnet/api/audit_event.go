package api

// audit_event.go:AuditEvent + AuditFilter 类型定义。

import (
	"strings"
	"time"
)

// AuditEvent 是审计日志中的一行。
//
// 刻意保持 schema 极简,以便写入器在高负载下零分配。
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`              // 时间戳
	Method    string    `json:"method"`                  // HTTP 方法
	Path      string    `json:"path"`                    // 请求路径
	KeyLabel  string    `json:"key_label,omitempty"`     // 使用的 API key 标签
	Role      string    `json:"role,omitempty"`          // 鉴权角色
	Tenant    string    `json:"tenant,omitempty"`        // 租户 ID
	Status    int       `json:"status"`                  // HTTP 状态码
	BytesIn   int64     `json:"bytes_in"`                // 请求体字节数
	BytesOut  int64     `json:"bytes_out"`               // 响应体字节数
	RemoteIP  string    `json:"remote_ip,omitempty"`     // 客户端 IP
	UserAgent string    `json:"user_agent,omitempty"`    // User-Agent
}

// AuditFilter 是 Filter 的查询 DSL。
//
// 空字段表示 "任意";所有非空字段按 AND 组合。
type AuditFilter struct {
	Method    string    // 精确匹配 HTTP 方法
	Path      string    // 子串匹配路径
	KeyLabel  string    // 精确匹配 key label
	Tenant    string    // 精确匹配租户
	StatusMin int       // 状态码下限(>0 才生效)
	StatusMax int       // 状态码上限(>0 才生效)
	Since     time.Time // 起始时间(零值表示不限)
	Until     time.Time // 截止时间(零值表示不限)
}

// matches 判断事件 e 是否匹配过滤器 f。
func (f AuditFilter) matches(e AuditEvent) bool {
	if f.Method != "" && e.Method != f.Method {
		return false
	}
	if f.Path != "" && !strings.Contains(e.Path, f.Path) {
		return false
	}
	if f.KeyLabel != "" && e.KeyLabel != f.KeyLabel {
		return false
	}
	if f.Tenant != "" && e.Tenant != f.Tenant {
		return false
	}
	if f.StatusMin > 0 && e.Status < f.StatusMin {
		return false
	}
	if f.StatusMax > 0 && e.Status > f.StatusMax {
		return false
	}
	if !f.Since.IsZero() && e.Timestamp.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && e.Timestamp.After(f.Until) {
		return false
	}
	return true
}
