package api

import (
	"strconv"
)

// atoiDefault 返回 s 的整数值；若 s 为空或无效则返回 def。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
