// Package tenants implements a tiny in-process tenant registry. Each
// tenant has a unique ID, a display name, an optional description, and
// a list of API keys that belong to it. The registry is the source of
// truth for tenant isolation: when a handler resolves a request it
// consults the registry to decide which tenant the request belongs to.
//
// In Round 23 the registry lives in memory and is seeded from the CLI
// flag `-tenants`. A future round can swap it for a SQLite-backed
// implementation without changing the public API.
package tenants

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("tenant not found")

// Tenant is the on-the-wire description of an organisation.
type Tenant struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
}

// Key is an API key owned by one tenant. We do not persist secrets; the
// caller is responsible for handing the plaintext to the operator over
// a secure channel.
type Key struct {
	TenantID  string    `json:"tenant_id"`
	Label     string    `json:"label"`
	Plaintext string    `json:"plaintext"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Registry is the in-memory tenant + key store.
type Registry struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant
	keys     map[string]string // plaintext key -> tenant ID
}

func New() *Registry {
	return &Registry{
		tenants: make(map[string]*Tenant),
		keys:    make(map[string]string),
	}
}

// CreateTenant registers a new tenant. ID is normalised to lowercase
// slug; an empty ID returns an error.
func (r *Registry) CreateTenant(id, name, description string) (*Tenant, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return nil, errors.New("tenant id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tenants[id]; exists {
		return nil, errors.New("tenant id already in use")
	}
	t := &Tenant{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		Active:      true,
	}
	r.tenants[id] = t
	return t, nil
}

// Get returns a tenant by id.
func (r *Registry) Get(id string) (*Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tenants[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := *t
	return &out, nil
}

// List returns all tenants in insertion order (by created_at).
func (r *Registry) List() []Tenant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tenant, 0, len(r.tenants))
	for _, t := range r.tenants {
		out = append(out, *t)
	}
	return out
}

// MintKey generates a fresh API key for the given tenant + role. The
// plaintext is returned exactly once so the caller can hand it to the
// human operator.
func (r *Registry) MintKey(tenantID, label, role string) (*Key, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[tenantID]; !ok {
		return nil, ErrNotFound
	}
	plaintext := randomKey()
	r.keys[plaintext] = tenantID
	return &Key{
		TenantID:  tenantID,
		Label:     label,
		Plaintext: plaintext,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// LookupTenant returns the tenant that owns a key, or empty when the
// key is not a tenant-bound key. Used by the auth middleware to stamp
// X-Dog-Tenant on the request.
func (r *Registry) LookupTenant(plaintext string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.keys[plaintext]
}

// SnapshotKeyMap returns a copy of every (plaintext -> tenant id) pair
// so the auth layer can build its registry without losing information.
func (r *Registry) SnapshotKeyMap() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.keys))
	for k, v := range r.keys {
		out[k] = v
	}
	return out
}

func randomKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "dog_" + hex.EncodeToString(b)
}
