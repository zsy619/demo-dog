package vault

// entry.go:Entry + AuditEntry 类型定义。

import "time"

// Entry 是存储中的一个秘密。
type Entry struct {
	Tenant     string    // 租户 ID
	Name       string    // 秘密名
	Ciphertext []byte    // 加密后的密文
	Nonce      []byte    // GCM 随机 nonce
	Version    int       // 写入版本号
	CreatedAt  time.Time // 创建时间
	UpdatedAt  time.Time // 最近更新时间
	CreatedBy  string    // 创建者
}

// AuditEntry 记录一次 Get/Put/Delete 操作。
type AuditEntry struct {
	Tenant string    // 租户 ID
	Name   string    // 秘密名
	Actor  string    // 操作者
	Action string    // 操作类型(get/put/delete)
	At     time.Time // 时间
	Found  bool      // 目标是否存在
}
