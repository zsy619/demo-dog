package api

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
)

// Role is a coarse-grained authorisation level. The collector maps
// roles to capabilities so a single API-key table can serve three
// distinct personas:
//
//   - admin:   full read + write + admin endpoints (rotate keys,
//              dump audit, hot-reload config).
//   - writer:  ingest + read. The default for any service-side SDK
//              that emits telemetry.
//   - reader:  read-only. Dashboard users and CI smoke probes.
type Role int

const (
	RoleReader Role = iota
	RoleWriter
	RoleAdmin
)

// String renders a role as a stable lowercase token so audit log
// lines and /api/keys responses stay human-readable.
func (r Role) String() string {
	switch r {
	case RoleAdmin:
		return "admin"
	case RoleWriter:
		return "writer"
	default:
		return "reader"
	}
}

// ParseRole accepts the canonical lowercase token and tolerates a
// few common variants ("r", "w", "ro", "rw"). Unknown values fall
// back to RoleReader because denying is safer than granting when
// the configuration is mis-typed.
func ParseRole(s string) Role {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "admin", "a":
		return RoleAdmin
	case "writer", "w", "rw":
		return RoleWriter
	case "reader", "r", "ro", "":
		return RoleReader
	default:
		return RoleReader
	}
}

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

// ErrForbidden is returned when the key is valid but lacks the
// role required for the operation (e.g. a reader trying to ingest).
var ErrForbidden = errors.New("role does not permit this operation")

// KeyEntry is one row of the auth registry. The Label field is the
// human-facing identifier (typically the service name or the user
// name). The Role field gates which capabilities the key carries.
type KeyEntry struct {
	Key   string
	Label string
	Role  Role
}

// APIKeyAuth is a tiny registry of accepted keys. Lookups use
// constant-time compare so two keys of the same length do not leak
// position-by-position through timing differences.
type APIKeyAuth struct {
	mu     sync.RWMutex
	keys   map[string]struct{}
	labels map[string]string
	roles  map[string]Role
}

// NewAPIKeyAuth returns an empty auth registry.
func NewAPIKeyAuth() *APIKeyAuth {
	return &APIKeyAuth{
		keys:   make(map[string]struct{}),
		labels: make(map[string]string),
		roles:  make(map[string]Role),
	}
}

// Add registers a new key with an optional label and a role.
// Empty keys are ignored so callers do not accidentally create a
// wildcard. When `role` is omitted the default is RoleWriter so the
// most common case (an SDK pushing telemetry) Just Works.
func (a *APIKeyAuth) Add(key, label string, role ...Role) {
	if key == "" {
		return
	}
	r := RoleWriter
	if len(role) > 0 {
		r = role[0]
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keys[key] = struct{}{}
	a.labels[key] = label
	a.roles[key] = r
}

// AddFromSpec accepts a parsed "<key>:<role>:<label>" or "<key>:<role>"
// or "<key>" record. This is the shape the CLI flag parser emits
// from `-api-keys k1:admin:alice,k2:writer:checkout,k3`. Unknown
// roles fall back to RoleWriter.
func (a *APIKeyAuth) AddFromSpec(spec string) {
	parts := strings.SplitN(spec, ":", 3)
	switch len(parts) {
	case 1:
		a.Add(parts[0], "", RoleWriter)
	case 2:
		a.Add(parts[0], "", ParseRole(parts[1]))
	default:
		a.Add(parts[0], parts[2], ParseRole(parts[1]))
	}
}

// Remove deletes a key from the registry.
func (a *APIKeyAuth) Remove(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.keys, key)
	delete(a.labels, key)
	delete(a.roles, key)
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

// RoleOf returns the role attached to a registered key. The bool is
// false when the key is not registered; callers should call Verify
// first and branch on the role only when the key is known.
func (a *APIKeyAuth) RoleOf(key string) (Role, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	r, ok := a.roles[key]
	return r, ok
}

// LabelOf returns the human-readable label for a registered key.
func (a *APIKeyAuth) LabelOf(key string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.labels[key]
}

// List returns a snapshot of every registered key. The returned
// slice is safe for the caller to mutate.
func (a *APIKeyAuth) List() []KeyEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]KeyEntry, 0, len(a.keys))
	for k := range a.keys {
		out = append(out, KeyEntry{
			Key:   k,
			Label: a.labels[k],
			Role:  a.roles[k],
		})
	}
	return out
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
			// Stash the resolved role + label on the context so
			// downstream handlers can branch without re-querying
			// the registry.
			if role, ok := a.RoleOf(key); ok {
				r.Header.Set("X-Dog-Role", role.String())
			}
			if label := a.LabelOf(key); label != "" {
				r.Header.Set("X-Dog-Key-Label", label)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole wraps a handler so it returns 403 unless the request
// carries an API key whose role is >= `min`. RoleAdmin outranks
// RoleWriter which outranks RoleReader. The middleware itself
// already enforces auth; this is the capability gate on top of it.
func RequireRole(min Role, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !roleAtLeast(r, min) {
			writeError(w, http.StatusForbidden, ErrForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// roleAtLeast reads the role header that Middleware stamps on the
// request and compares it to the required minimum. Unknown roles
// (e.g. when auth is off) default to RoleReader.
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
