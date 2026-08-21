package jwt

// context.go:基于 context.Context 的 claims 注入与读取。
//
// claimsCtx 包装父 context, 通过 ctxKey 提供 claims/kid 的间接访问,
// 避免污染 context 的全局命名空间。

import (
	"context"
	"time"
)

// ctxKey 是 context 键类型(避免与其他包冲突)。
type ctxKey int

const (
	claimsKey ctxKey = iota // claims 映射的键
	kidKey                 // kid 字符串的键
)

// claimsCtx 把 claims 与 kid 包装到一个 context.Context。
type claimsCtx struct {
	ctx    context.Context // 父 context
	claims map[string]any  // 解析后的 claims
	kid    string          // 签名 key ID
}

// Value 返回指定 key 的值,未识别则透传给父 context。
func (c *claimsCtx) Value(key any) any {
	switch key {
	case claimsKey:
		return c.claims
	case kidKey:
		return c.kid
	}
	return c.ctx.Value(key)
}

// Deadline 透传父 context。
func (c *claimsCtx) Deadline() (deadline time.Time, ok bool) { return c.ctx.Deadline() }

// Done 透传父 context。
func (c *claimsCtx) Done() <-chan struct{} { return c.ctx.Done() }

// Err 透传父 context。
func (c *claimsCtx) Err() error { return c.ctx.Err() }

// WithClaims 将 claims 与 kid 包装到 context.Context 中。
func WithClaims(ctx context.Context, claims map[string]any, kid string) context.Context {
	return &claimsCtx{ctx: ctx, claims: claims, kid: kid}
}

// FromContext 返回由中间件注入到上下文中的 claims 映射
// 与 kid。
func FromContext(ctx context.Context) (map[string]any, string, bool) {
	c, ok := ctx.Value(claimsKey).(map[string]any)
	if !ok {
		return nil, "", false
	}
	kid, _ := ctx.Value(kidKey).(string)
	return c, kid, true
}
