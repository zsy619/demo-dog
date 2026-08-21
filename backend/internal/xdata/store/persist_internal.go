package store

// persist_internal.go:persist 相关私有辅助函数。

import "github.com/zsy619/demo-dog/backend/internal/xdata/model"

// copyPersistMV 深拷贝一个 MV bucket map(nil 安全)。
func copyPersistMV(src map[string][]model.MVBucket) map[string][]model.MVBucket {
	if src == nil {
		return nil
	}
	out := make(map[string][]model.MVBucket, len(src))
	for k, v := range src {
		out[k] = append([]model.MVBucket(nil), v...)
	}
	return out
}
