package api

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
)

// AuthMode controls how incoming requests are authenticated.
//
//   - AuthModeOff:    no auth (default for dev). Server starts in this mode
//                     if no keys are configured.
//   - AuthModeAPIKey: every request must carry one of the registered keys.
type AuthMode int

const (
	AuthModeOff AuthMode = iota
	AuthModeAPIKey
)

// ErrUnauthorized is returned by the auth middleware. We keep a
// package-level sentinel so callers can branch on it without string
// matching.
var ErrUnauthorized = errors.New("missing or invalid API key")

// APIKeyAuth is a tiny registry of accepted keys. Lookups use
// constant-time compare so two keys of the same length do not leak
// position-by-position through timing differences.
//
// The registry is intentionally minimal — no rotation / revocation
// / TTL yet. That will be added when we wire multi-tenant identity.
type APIKeyAuth struct {
	mu     sync.RWMutex
	keys   map[string]struct{}
	labels map[string]string
}

// NewAPIKeyAuth returns an empty auth registry.
func NewAPIKeyAuth() *APIKeyAuth {
	return &APIKeyAuth{
		keys:   make(map[string]struct{}),
		labels: make(map[string]string),
	}
}

// Add registers a new key with an optional human label. Empty keys are
// ignored so callers do not accidentally create a wildcard.
func (a *APIKeyAuth) Add(key, label string) {
	if key == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keys[key] = struct{}{}
	a.labels[key] = label
}

// Remove deletes a key from the registry.
func (a *APIKeyAuth) Remove(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.keys, key)
	delete(a.labels, key)
}

// Count returns the number of registered keys.
func (a *APIKeyAuth) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.keys)
}

// Verify returns true if `key` matches one of the registered keys.
// Uses crypto/subtle.ConstantTimeCompare on candidate pairs of the
// same length to resist timing-side-channel attacks. Empty registered
// sets return false.
func (a *APIKeyAuth) Verify(key string) bool {
	if key == "" {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for candidate := range a.keys {
		if len(candidate) != len(key) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(key)) == 1 {
			return true
		}
	}
	return false
}

// Middleware returns an http middleware that enforces `mode`. When
// mode == AuthModeOff the middleware is a pass-through (useful for
// dev deployments where auth is intentionally disabled).
//
// PublicPaths are skipped entirely so /api/health and /metrics always
// respond for liveness probes and Prometheus scrapers.
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
			next.ServeHTTP(w, r)
		})
	}
}

// extractKey pulls the API key from the request. Accepts the
// standard "Authorization: Bearer <token>" header as well as the
// legacy "X-API-Key" header. Falls back to ?api_key=... for browser
// debugging only (never use for production traffic).
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
