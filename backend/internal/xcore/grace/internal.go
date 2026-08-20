package grace

// internal.go：私有辅助函数（不对外暴露）。

import (
	"context"
	"fmt"
	"strings"
)

// safeRun 在 panic 时返回错误，避免 Hook 抛 panic 拖垮 Shutdown 协调器。
//
// 返回值 err 会被 defer 中的 recover 设置为 panic 描述。
func safeRun(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx)
}

// joinErrors 拼接多个错误为单行字符串（用 "; " 分隔）。
//
// 用于在最终错误提示中展示所有 Hook 失败信息。
func joinErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	parts := make([]string, len(errs))
	for i, e := range errs {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "; ")
}
