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

// Auth is a bearer-token authenticator for /replica endpoints.
//
// Wire format:
//
//	Authorization: Bearer <token>
type Auth struct {
	mu     sync.RWMutex
	tokens map[string]string
}

// NewAuth creates an Auth with token:follower-id entries.
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

// Enabled reports whether auth is enforced.
func (a *Auth) Enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.tokens) > 0
}

// Middleware wraps an http.Handler with bearer-token enforcement.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.Authenticate(r) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Authenticate returns the follower ID on success or empty string on failure.
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

// TLSConfigFromPairs builds a *tls.Config from PEM bytes.
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
