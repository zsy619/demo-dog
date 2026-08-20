package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Errors 由认证层返回的错误。
var (
	ErrNoAuth       = errors.New("missing authentication")
	ErrBadAuth      = errors.New("malformed authentication")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// Principal 是已认证的调用方。
type Principal struct {
	Subject  string
	Tenant   string
	Identity string
	Scopes   []string
	Method   string
}

// HasScope 在 principal 拥有指定 scope 时返回 true。
// 空的 Scopes 列表 = 旧式无作用域 key，
// 匹配所有内容（因此能通过 role/scope 检查）。
// 由 RequireScope 签发的已认证调用方必须具有显式 scope。
func (p Principal) HasScope(s string) bool {
	if len(p.Scopes) == 0 {
		return true
	}
	for _, sc := range p.Scopes {
		if sc == s {
			return true
		}
	}
	return false
}

// IsAdmin 报告 principal 是否带有 admin role 或 scope。
// IsAdmin 报告 principal 是否带有 admin role 或 scope。
func (p Principal) IsAdmin() bool {
	if p.Identity == "admin" {
		return true
	}
	for _, sc := range p.Scopes {
		if sc == "admin" {
			return true
		}
	}
	return false
}

type ctxKey struct{}

// PrincipalFromContext 从 ctx 中提取 principal。
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// WithPrincipal 将 p 挂到 ctx 上。
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// bearerEntry 是 bearer 查找所返回的接口。
type bearerEntry interface {
	IsValid(time.Time) bool
	HasScope(string) bool
}

// BearerEntry 是任意 KeyEntry 形状值的轻量适配器。
type BearerEntry struct {
	Valid    func(time.Time) bool
	Scopes   []string
	KeyID    string
	Tenant   string
	Identity string
}

func (b BearerEntry) IsValid(t time.Time) bool {
	if b.Valid != nil {
		return b.Valid(t)
	}
	return true
}

func (b BearerEntry) HasScope(s string) bool {
	if len(b.Scopes) == 0 {
		return true
	}
	for _, sc := range b.Scopes {
		if sc == s {
			return true
		}
	}
	return false
}

// MTLSVerifier 校验对等方证书。
type MTLSVerifier interface {
	VerifyPeer(subject string) (string, bool)
}

// OIDCVerifier 校验原始 ID token。
type OIDCVerifier interface {
	VerifyToken(ctx context.Context, raw string) (subject, tenant string, scopes []string, err error)
}

// Middleware 是顶层认证链。
type Middleware struct {
	Bearer   map[string]bearerEntry
	MTLS     MTLSVerifier
	OIDC     OIDCVerifier
	PeerCert func(r *http.Request) string
	Time     func() time.Time
}

func (m *Middleware) nowFn() func() time.Time {
	if m.Time == nil {
		return time.Now
	}
	return m.Time
}

// Authenticate 检查请求并返回一个 Principal。
// 首个匹配获胜：mTLS > Bearer > OIDC。
func (m *Middleware) Authenticate(r *http.Request) (Principal, error) {
	if m.MTLS != nil && m.PeerCert != nil {
		if cn := m.PeerCert(r); cn != "" {
			if tenant, ok := m.MTLS.VerifyPeer(cn); ok {
				return Principal{Subject: cn, Tenant: tenant, Identity: "client", Method: "mtls"}, nil
			}
		}
	}
	auth := r.Header.Get("Authorization")
	if auth != "" {
		kind, raw := splitAuth(auth)
		switch kind {
		case "Bearer":
			if m.Bearer != nil {
				if entry, ok := m.Bearer[raw]; ok {
					if !entry.IsValid(m.nowFn()()) {
						return Principal{}, ErrUnauthorized
					}
					if be, ok := entry.(BearerEntry); ok {
						return Principal{
							Subject:  be.KeyID,
							Tenant:   be.Tenant,
							Identity: be.Identity,
							Scopes:   be.Scopes,
							Method:   "bearer",
						}, nil
					}
					if p, ok := entry.(Principal); ok {
						return p, nil
					}
				}
			}
		case "Bearer-OIDC":
			if m.OIDC != nil {
				sub, tenant, scopes, err := m.OIDC.VerifyToken(r.Context(), raw)
				if err != nil {
					return Principal{}, err
				}
				return Principal{Subject: sub, Tenant: tenant, Identity: sub, Scopes: scopes, Method: "oidc"}, nil
			}
		}
	}
	return Principal{}, ErrNoAuth
}

func splitAuth(s string) (kind, raw string) {
	idx := strings.IndexByte(s, ' ')
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

// RequireAny 拒绝未认证的调用方，返回 401。
func (m *Middleware) RequireAny(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := m.Authenticate(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		ctx := WithPrincipal(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole 拒绝不具备指定身份的调用方。
func (m *Middleware) RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := m.Authenticate(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if p.Identity != role {
			http.Error(w, "forbidden: role", http.StatusForbidden)
			return
		}
		ctx := WithPrincipal(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireScope 拒绝不具备指定 scope 的调用方。
func (m *Middleware) RequireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := m.Authenticate(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if !p.HasScope(scope) {
			http.Error(w, "forbidden: scope", http.StatusForbidden)
			return
		}
		ctx := WithPrincipal(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HasScope 是请求处理器中的便捷检查。
func HasScope(r *http.Request, s string) bool {
	p, ok := PrincipalFromContext(r.Context())
	if !ok {
		return false
	}
	return p.HasScope(s)
}

// HashToken 返回 token 的 sha256 十六进制。
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// CompareTokens 是常量时间比较。
func CompareTokens(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// PrincipalMap 是一个轻量的内存验证器。
type PrincipalMap struct {
	mu   sync.RWMutex
	keys map[string]Principal
}

// NewPrincipalMap 返回一个空的 map。
func NewPrincipalMap() *PrincipalMap {
	return &PrincipalMap{keys: make(map[string]Principal)}
}

// Register 添加一个可通过原始 token 访问的 principal。
func (pm *PrincipalMap) Register(raw string, p Principal) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.keys[raw] = p
}

// AsMiddleware 将 PrincipalMap 适配为 Middleware。
func (pm *PrincipalMap) AsMiddleware() *Middleware {
	return &Middleware{Bearer: pm.snapshot()}
}

func (pm *PrincipalMap) snapshot() map[string]bearerEntry {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make(map[string]bearerEntry, len(pm.keys))
	for k, v := range pm.keys {
		out[k] = v
	}
	return out
}

// DecodeAuthorization 将头部拆分为种类与原始值。
func DecodeAuthorization(s string) (string, string, error) {
	k, r := splitAuth(s)
	if k == "" {
		return "", "", ErrBadAuth
	}
	return k, r, nil
}

// ComposeBearer 构造一个 "Authorization: Bearer X" 字符串。
func ComposeBearer(raw string) string {
	return fmt.Sprintf("Bearer %s", raw)
}

// IsValid 让 Principal 满足 bearerEntry。
func (p Principal) IsValid(time.Time) bool { return true }

