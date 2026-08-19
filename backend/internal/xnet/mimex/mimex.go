// Package mimex 提供一个轻量的 MIME 类型推断。
package mimex

import (
	"path/filepath"
	"strings"
)

var table = map[string]string{
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
	".js":   "application/javascript",
	".json": "application/json",
	".xml":  "application/xml",
	".txt":  "text/plain",
	".csv":  "text/csv",
	".md":   "text/markdown",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
	".pdf":  "application/pdf",
	".zip":  "application/zip",
	".tar":  "application/x-tar",
	".gz":   "application/gzip",
	".mp3":  "audio/mpeg",
	".mp4":  "video/mp4",
	".wasm": "application/wasm",
}

// FromExt 返回扩展名对应的 MIME，缺失时返回 def。
func FromExt(ext, def string) string {
	ext = strings.ToLower(ext)
	if v, ok := table[ext]; ok {
		return v
	}
	return def
}

// FromFile 返回文件名对应的 MIME。
func FromFile(name, def string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return def
	}
	return FromExt(ext, def)
}

// Register 允许运行时注册新映射。
func Register(ext, mime string) {
	table[strings.ToLower(ext)] = mime
}
