package api

// role.go：Role 类型与角色解析。
//
// Role 是粗粒度的授权层级；收集器将角色映射到能力，
// 使得一个 API Key 表能够服务于三种典型角色。
//
// 角色层级：
//   - admin：完整读写 + 管理端点（轮换密钥、导出审计、热加载配置）；
//   - writer：写入 + 读取；默认用于服务端 SDK 上报数据；
//   - reader：只读；用于面板用户与 CI 烟雾探测。

import "strings"

// Role 是粗粒度的授权层级。
//
// 收集器将角色映射到能力，使得单一 API Key 表能够服务三种角色：
//   - admin：完整读写 + 管理端点（轮换密钥、导出审计、热加载配置）；
//   - writer：写入 + 读取；默认用于服务端 SDK 上报遥测；
//   - reader：只读；面板用户与 CI 烟雾探测。
type Role int

const (
	// RoleReader 只读角色。
	RoleReader Role = iota
	// RoleWriter 写读角色（默认用于 SDK 上报）。
	RoleWriter
	// RoleAdmin 全权管理员角色。
	RoleAdmin
)

// String 将 Role 渲染为稳定的小写 token，便于审计日志与 /api/keys 响应可读。
func (r Role) String() string {
	switch r {
	case RoleAdmin:
		return "admin"
	case RoleWriter:
		return "writer"
	default:
		return "reader"
	}
}

// ParseRole 接受规范的小写 token，并容忍几种常见简写（"r"、"w"、"ro"、"rw"）。
//
// 未未知值 fall back 到 RoleReader，因为配置错误时拒绝访问比授予权限更安全。
func ParseRole(s string) Role {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "admin", "a":
		return RoleAdmin
	case "writer", "w", "rw":
		return RoleWriter
	case "reader", "r", "ro", "":
		return RoleReader
	default:
		return RoleReader
	}
}
