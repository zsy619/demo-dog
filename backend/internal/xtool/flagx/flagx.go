// Package flagx 提供从环境变量 / 文件 / 命令行获取字符串、整型、布尔值的辅助函数。
package flagx

import (
	"os"
	"strconv"
)

// GetEnv 返回环境变量，缺失返回 def。
func GetEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// GetEnvInt 返回整型环境变量。
func GetEnvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// GetEnvBool 返回布尔型环境变量。
func GetEnvBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	switch v {
	case "1", "t", "T", "true", "TRUE", "True":
		return true
	case "0", "f", "F", "false", "FALSE", "False":
		return false
	}
	return def
}

// FileOrDefault 优先读取文件，失败返回 def。
func FileOrDefault(path, def string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	return string(b)
}

// MustEnv 必须存在环境变量，否则 panic。
func MustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic("flagx: 必需的环境变量 " + k + " 未设置")
	}
	return v
}
