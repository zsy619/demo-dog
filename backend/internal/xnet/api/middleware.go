package api

// middleware.go：HTTP 鉴权中间件与角色门控。
//
// APIKeyAuth.Middleware 是核心中间件（每次请求都执行）；
// RequireRole 是细粒度的角色门控中间件（在路由层叠加）。

import (
	"net/http"
	"strings"
)

// Middleware 返回一个强制 mode 鉴权的 http 中间件。
//
// 当 mode == AuthModeOff 时中间件直通（用于开发环境刻意关闭鉴权）。
//
// PublicPaths 会被完全跳过，
// 保证 /api/health 与 /metrics 始终响应存活探测与 Prometheus 抓取。
func (a *APIKeyAuth) Middleware(mode AuthMode, publicPaths ...string) func(http.Handler) http.Handler {
	pub := make(map[string]bool, len(publicPaths))
	for _, p := range publicPaths {
		pub[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mode == AuthModeOff {
				next.ServeHTTP(w, r)
				return
			}
			if pub[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			key := extractKey(r)
			if !a.Verify(key) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="dog-collector"`)
				writeError(w, http.StatusUnauthorized, ErrUnauthorized)
				return
			}
			// 把已解析的角色 / 标签 / 租户塞到 Header，
			// 让下游 handler 无需再查询注册表。
			if role, ok := a.RoleOf(key); ok {
				r.Header.Set("X-Dog-Role", role.String())
			}
			if label := a.LabelOf(key); label != "" {
				r.Header.Set("X-Dog-Key-Label", label)
			}
			if tenant := a.TenantOf(key); tenant != "" {
				r.Header.Set("X-Dog-Tenant", tenant)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole 包装一个 handler，除非请求携带的角色 ≥ min，否则返回 403。
//
// 角色层级：RoleAdmin > RoleWriter > RoleReader。
// 该函数用于在鉴权中间件之上叠加更细粒度的能力门控。
func RequireRole(min Role, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !roleAtLeast(r, min) {
			writeError(w, http.StatusForbidden, ErrForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// roleAtLeast 读取 Middleware 写入的 role header 并与最低角色比较。
//
// 未未知角色（例如关闭鉴权时）默认为 RoleReader。
func roleAtLeast(r *http.Request, min Role) bool {
	switch r.Header.Get("X-Dog-Role") {
	case "admin":
		return min <= RoleAdmin
	case "writer":
		return min <= RoleWriter
	default:
		return min <= RoleReader
	}
}

// extractKey 从请求中抽取 API key。
//
// 优先级：
//  1. 标准 "Authorization: Bearer <token>" 头；
//  2. 旧版 "X-API-Key" 头；
//  3. ?api_key=... 查询参数（仅供浏览器调试，生产环境禁用）。
func extractKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		}
		return strings.TrimSpace(h)
	}
	if k := r.Header.Get("X-API-Key"); k != "" {
		return strings.TrimSpace(k)
	}
	return strings.TrimSpace(r.URL.Query().Get("api_key"))
}
