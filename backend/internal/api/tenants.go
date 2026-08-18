package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zsy619/demo-dog/backend/internal/tenants"
)

// handleTenantsList returns every tenant. Admin-only.
func (s *Server) handleTenantsList(w http.ResponseWriter, r *http.Request) {
	if s.tenants == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tenants": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenants": s.tenants.List()})
}

type createTenantReq struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleTenantCreate registers a new tenant. Admin-only.
func (s *Server) handleTenantCreate(w http.ResponseWriter, r *http.Request) {
	if s.tenants == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("tenants disabled"))
		return
	}
	var req createTenantReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	t, err := s.tenants.CreateTenant(req.ID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

type mintKeyReq struct {
	Label string `json:"label"`
	Role  string `json:"role"`
}

// handleTenantMintKey generates a fresh API key for a tenant.
// The plaintext is returned exactly once.
func (s *Server) handleTenantMintKey(w http.ResponseWriter, r *http.Request) {
	if s.tenants == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("tenants disabled"))
		return
	}
	tenantID := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	tenantID = strings.TrimSuffix(tenantID, "/keys")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("tenant id required"))
		return
	}
	var req mintKeyReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Label == "" {
		req.Label = "default"
	}
	if req.Role == "" {
		req.Role = "writer"
	}
	k, err := s.tenants.MintKey(tenantID, req.Label, req.Role)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	// Register the freshly minted key in the auth layer.
	s.auth.AddForTenant(k.Plaintext, k.Label, tenantID, ParseRole(k.Role))
	writeJSON(w, http.StatusCreated, k)
}

// resolveTenant inspects the incoming request and returns the tenant
// the caller is acting on. Resolution order:
//
//   1. The X-Dog-Tenant header stamped by the auth middleware (when
//      the API key is bound to a tenant).
//   2. The ?tenant=... query parameter (used by admin impersonation).
//
// Admins may pass an explicit tenant via the query parameter to
// impersonate; non-admins are pinned to whatever tenant their key is
// bound to (or empty when their key is unbound, which is platform-admin
// territory).
func resolveTenant(r *http.Request) string {
	if t := r.Header.Get("X-Dog-Tenant"); t != "" {
		return t
	}
	return r.URL.Query().Get("tenant")
}

var _ = subtle.ConstantTimeCompare
var _ = tenants.ErrNotFound

// handleTenantsRoute dispatches /api/tenants/<id>/keys to the
// minter. Anything else under /api/tenants/ returns 404.
func (s *Server) handleTenantsRoute(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	if strings.HasSuffix(suffix, "/keys") {
		s.handleTenantMintKey(w, r)
		return
	}
	http.NotFound(w, r)
}

// handleTenantsDispatch serves both /api/tenants (list + create) and
// /api/tenants/<id>/keys (mint). The Go stdlib mux does not pattern
// match suffixes on a single HandleFunc so we dispatch by URL shape.
func (s *Server) handleTenantsDispatch(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/tenants":
		if r.Method == http.MethodPost {
			s.handleTenantCreate(w, r)
			return
		}
		s.handleTenantsList(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/keys") {
		s.handleTenantMintKey(w, r)
		return
	}
	http.NotFound(w, r)
}

// TrimPrefix in handleTenantMintKey needs an updated path because
// the route prefix may include leading whitespace. Re-derive from
// r.URL.Path inside the handler.
