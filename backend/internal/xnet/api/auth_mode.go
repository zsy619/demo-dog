package api

// auth_mode.go：AuthMode 与哨兵错误。
//
// AuthMode 控制入站请求的鉴权方式；哨兵错误供中间件返回。

import "errors"

// AuthMode 控制入站请求的鉴权方式。
//
//   - AuthModeOff：关闭鉴权（开发模式默认）；
//   - AuthModeAPIKey：每个请求必须携带一个已注册的密钥。
type AuthMode int

const (
	// AuthModeOff 表示关闭鉴权。
	AuthModeOff AuthMode = iota
	// AuthModeAPIKey 表示启用 API Key 鉴权。
	AuthModeAPIKey
)

// ErrUnauthorized 是鉴权中间件返回的哨兵错误。
//
// 使用包级哨兵错误，调用方可通过 errors.Is 判断而无需字符串匹配。
var ErrUnauthorized = errors.New("missing or invalid API key")

// ErrForbidden 在密钥有效但缺少所需角色权限时返回（例如 reader 试图写入）。
var ErrForbidden = errors.New("role does not permit this operation")
