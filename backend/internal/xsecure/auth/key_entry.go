// Package auth 管理 API key 轮换与租户 CRUD 的辅助方法。
//
// 包含：
//   - key_entry.go   KeyEntry 类型及其能力判定方法
//   - token.go       密码学 token 生成与哈希
//   - admin_store.go AdminStore 主体（key 表的 CRUD + 轮换）
//
// Key rotation lets an operator mint a new key, mark the old
// one as expiring (still valid for a grace window), and
// eventually delete it. The auth layer is unaffected: tokens
// continue to match by hash.
//
// Tenant helpers provide CRUD + list operations for
// /api/admin/tenants. The full registry lives in
// xdata/tenants; this package re-exports the operations
// the admin handler wires up.
package auth

import "time"

// KeyEntry 是认证表的一行记录。
//
// 唯一标识符是 KeyID；Hash 用于根据原始 token 查找；
// Identity 表示角色（role:admin / role:reader 等）。
type KeyEntry struct {
	KeyID       string    // 唯一标识符
	Hash        string    // raw token 的 sha256 hex
	Identity    string    // 角色（如 role:admin / role:reader）
	Tenant      string    // 绑定的租户
	Scopes      []string  // 授权范围
	CreatedAt   time.Time // 创建时间
	ExpiresAt   time.Time // 过期时间（零值表示永不过期）
	Disabled    bool      // 是否已禁用
	RotatedFrom string    // 被新 key 替换时填入旧 KeyID
}

// IsValid 报告该条目当前是否可用。
//
// 任一条件不满足则返回 false：
//   - Disabled == true；
//   - ExpiresAt 已过（now 之后）。
func (e *KeyEntry) IsValid(now time.Time) bool {
	if e.Disabled {
		return false
	}
	if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
		return false
	}
	return true
}

// HasScope 报告该条目是否拥有指定的 scope。
//
// 空的 Scopes 列表表示 legacy 无限制，匹配任意 ACL。
func (e *KeyEntry) HasScope(s string) bool {
	if len(e.Scopes) == 0 {
		return true
	}
	for _, sc := range e.Scopes {
		if sc == s {
			return true
		}
	}
	return false
}

// HasResourceScope 是按资源维度的变体，供 /api/v1/rules、/api/v1/quotas 等使用。
//
// 匹配规则：拥有 "<resource>:read" 或 "<resource>:write" 任一 scope 即视为授权。
func (e *KeyEntry) HasResourceScope(resource string) bool {
	return e.HasScope(resource+":read") || e.HasScope(resource+":write")
}
