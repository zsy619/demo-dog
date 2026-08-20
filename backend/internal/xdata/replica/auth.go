package replica

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
)

// Auth 是 /replica 端点的 bearer-token 认证器。
//
// Wire format:
//
//	Authorization: Bearer <token>
type Auth struct {
	mu     sync.RWMutex
	tokens map[string]string
}

// NewAuth 创建带 token:follower-id 条目的 Auth。
func NewAuth(entries []string) *Auth {
	a := &Auth{tokens: make(map[string]string)}
	for _, e := range entries {
		parts := strings.SplitN(e, ":", 2)
		if len(parts) != 2 {
			continue
		}
		tok := strings.TrimSpace(parts[0])
		id := strings.TrimSpace(parts[1])
		if tok == "" || id == "" {
			continue
		}
		a.tokens[hashToken(tok)] = id
	}
	return a
}

// Enabled 报告是否强制认证。
func (a *Auth) Enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.tokens) > 0
}

// Middleware 用 bearer-token 强制包装 http.Handler。
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.Authenticate(r) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Authenticate 成功时返回 follower ID，失败时返回空字符串。
func (a *Auth) Authenticate(r *http.Request) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.tokens) == 0 {
		return "anon"
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	tok := strings.TrimPrefix(h, "Bearer ")
	id, ok := a.tokens[hashToken(tok)]
	if !ok {
		got := sha256.Sum256([]byte(tok))
		for kh := range a.tokens {
			expected, _ := hex.DecodeString(kh)
			subtle.ConstantTimeCompare(got[:], expected)
		}
		return ""
	}
	return id
}

// hashToken hashes the token so a memory dump does not leak them.
func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// TLSConfigFromPairs 从 PEM 字节构建 *tls.Config。
func TLSConfigFromPairs(certPEM, keyPEM []byte) (*tls.Config, error) {
	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{c},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
