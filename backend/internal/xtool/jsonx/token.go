// Package jsonx token.go: Token / TokenType / ErrBadJSON。
package jsonx

import "errors"

// TokenType 是 JSON token 的种类。
type TokenType int

const (
	TokenEOF TokenType = iota // 流结束
	TokenObjectStart          // {
	TokenObjectEnd            // }
	TokenArrayStart           // [
	TokenArrayEnd             // ]
	TokenString               // 字符串
	TokenNumber               // 数字
	TokenBool                 // true/false
	TokenNull                 // null
)

// ErrBadJSON 在语法错误时返回。
var ErrBadJSON = errors.New("bad json")

// Token 是一个已解析的 JSON 值。
//
// Type == TokenString 时使用 Str；
// Type == TokenNumber 时使用 Num；
// Type == TokenBool 时使用 Bool。
type Token struct {
	Type TokenType
	Str  string
	Num  float64
	Bool bool
}
