package api

// key_entry.go：KeyEntry 类型定义。
//
// KeyEntry 是鉴权注册表的一行记录，
// 包含标签、角色、租户绑定与可选的资源范围。

// KeyEntry 是鉴权注册表的一行记录。
//
// Label 是面向人类可读的标识符（通常是服务名或用户名）；
// Role 控制该密钥拥有的能力；
// TenantID 将密钥绑定到一个租户；空字符串表示可模拟任意租户（仅用于平台管理员密钥）。
//
// Scopes 是资源级范围集合：
//   - 空切片表示无限制，该密钥可访问其租户下的任意服务 / 指标；
//   - 非空切片表示该密钥只能访问名字在 Scopes 中出现的资源，
//     适用于将第三方集成限制在特定服务上。
type KeyEntry struct {
	Key      string   // API Key 原文（注册表内部使用）
	Label    string   // 面向人类可读的标识符
	Role     Role     // 角色
	TenantID string   // 租户绑定（空 = 不限租户）
	Scopes   []string // 资源级范围（空 = 不限资源）
}
