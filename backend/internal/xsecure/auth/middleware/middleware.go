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

// Errors returned by the auth layer.
var (
	ErrNoAuth       = errors.New("missing authentication")
	ErrBadAuth      = errors.New("malformed authentication")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// Principal is the authenticated caller.
type Principal struct {
	Subject  string
	Tenant   string
	Identity string
	Scopes   []string
	Method   string
}

// HasScope returns true if the principal has the named scope.
// Empty Scopes list = unscoped legacy keys that match
// everything (so they pass role/scope checks). Authenticated
// callers minted by RequireScope must have explicit scopes.
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

// IsAdmin reports whether the principal carries the admin
// role or scope.
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

// PrincipalFromContext extracts the principal from ctx.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// WithPrincipal attaches p to ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// bearerEntry is the interface returned by the bearer lookup.
type bearerEntry interface {
	IsValid(time.Time) bool
	HasScope(string) bool
}

// BearerEntry is a thin adapter for any KeyEntry-shaped value.
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

// MTLSVerifier checks a peer certificate.
type MTLSVerifier interface {
	VerifyPeer(subject string) (string, bool)
}

// OIDCVerifier validates a raw ID token.
type OIDCVerifier interface {
	VerifyToken(ctx context.Context, raw string) (subject, tenant string, scopes []string, err error)
}

// Middleware is the top-level auth chain.
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

// Authenticate inspects the request and returns a Principal.
// First match wins: mTLS > Bearer > OIDC.
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

// RequireAny rejects unauthenticated callers with 401.
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

// RequireRole rejects callers without the given identity.
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

// RequireScope rejects callers without the named scope.
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

// HasScope is a convenience check from the request handler.
func HasScope(r *http.Request, s string) bool {
	p, ok := PrincipalFromContext(r.Context())
	if !ok {
		return false
	}
	return p.HasScope(s)
}

// HashToken returns sha256 hex of the token.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// CompareTokens is constant-time comparison.
func CompareTokens(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// PrincipalMap is a small in-memory verifier.
type PrincipalMap struct {
	mu   sync.RWMutex
	keys map[string]Principal
}

// NewPrincipalMap returns an empty map.
func NewPrincipalMap() *PrincipalMap {
	return &PrincipalMap{keys: make(map[string]Principal)}
}

// Register adds a principal accessible by raw token.
func (pm *PrincipalMap) Register(raw string, p Principal) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.keys[raw] = p
}

// AsMiddleware adapts a PrincipalMap into a Middleware.
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

// DecodeAuthorization splits a header into kind + raw.
func DecodeAuthorization(s string) (string, string, error) {
	k, r := splitAuth(s)
	if k == "" {
		return "", "", ErrBadAuth
	}
	return k, r, nil
}

// ComposeBearer constructs an "Authorization: Bearer X" string.
func ComposeBearer(raw string) string {
	return fmt.Sprintf("Bearer %s", raw)
}

// IsValid lets Principal satisfy bearerEntry.
func (p Principal) IsValid(time.Time) bool { return true }

