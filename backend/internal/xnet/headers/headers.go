// Package headers 提供 HTTP 常用 Header 常量与工具。
package headers

import (
	"net/http"
	"strings"
)

// 通用 Header 常量。
const (
	Authorization = "Authorization"
	ContentType   = "Content-Type"
	UserAgent     = "User-Agent"
	Accept        = "Accept"
	AcceptEncoding = "Accept-Encoding"
	ContentLength = "Content-Length"
	Location      = "Location"
	RetryAfter    = "Retry-After"
	XRequestID    = "X-Request-ID"
	XForwardedFor = "X-Forwarded-For"
)

// CommonContentTypes 列出常用 Content-Type。
var CommonContentTypes = map[string]string{
	"json": "application/json",
	"xml":  "application/xml",
	"form": "application/x-www-form-urlencoded",
	"bin":  "application/octet-stream",
	"text": "text/plain",
}

// GetFirst 取出 header 第一个值。
func GetFirst(h http.Header, name string) string {
	return strings.TrimSpace(h.Get(name))
}

// Set 设置 header。
func Set(h http.Header, name, val string) {
	h.Set(name, val)
}

// Add 追加 header。
func Add(h http.Header, name, val string) {
	h.Add(name, val)
}

// ContentTypeOf 推断 content-type。
func ContentTypeOf(name string) string {
	switch strings.ToLower(name) {
	case "json":
		return CommonContentTypes["json"]
	case "xml":
		return CommonContentTypes["xml"]
	}
	return "application/octet-stream"
}
