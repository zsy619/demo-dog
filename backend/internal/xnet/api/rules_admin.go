package api

// Rules CRUD endpoints (Round 53 parity work).

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
)

func (s *Server) handleRulesAdmin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/rules/")
		if name == "" {
			writeError(w, http.StatusBadRequest, errors.New("name required"))
			return
		}
		rl, ok := s.alerts.getRule(name)
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("rule not found"))
			return
		}
		writeJSON(w, http.StatusOK, rl)
	case http.MethodPut:
		if !s.allowedFor(r, "rules:write") {
			writeError(w, http.StatusForbidden, errors.New("missing rules:write scope"))
			return
		}
		var rl alerts.Rule
		if err := json.NewDecoder(r.Body).Decode(&rl); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if rl.Name == "" {
			writeError(w, http.StatusBadRequest, errors.New("name required"))
			return
		}
		if rl.Window <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("window must be positive"))
			return
		}
		prev, replaced := s.alerts.upsertRule(rl)
		if replaced {
			writeJSON(w, http.StatusOK, map[string]any{
				"rule":     rl,
				"previous": prev,
				"created":  false,
			})
		} else {
			writeJSON(w, http.StatusCreated, map[string]any{
				"rule":    rl,
				"created": true,
			})
		}
	case http.MethodDelete:
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/rules/")
		if name == "" {
			writeError(w, http.StatusBadRequest, errors.New("name required"))
			return
		}
		if !s.allowedFor(r, "rules:write") {
			writeError(w, http.StatusForbidden, errors.New("missing rules:write scope"))
			return
		}
		deleted, ok := s.alerts.deleteRule(name)
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("rule not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
	default:
		w.Header().Set("Allow", "GET PUT DELETE")
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET PUT DELETE only"))
	}
}

func (s *Server) allowedFor(r *http.Request, scope string) bool {
	key := extractKey(r)
	if key == "" {
		// No key presented. AuthModeOff means anyone can write; the
		// production modes always require a key for write endpoints.
		return s.authM == AuthModeOff
	}
	if !s.auth.AllowsResource(key, scope) {
		return false
	}
	scopes := s.auth.ScopesFor(key)
	if len(scopes) == 0 {
		return true
	}
	for _, sc := range scopes {
		if sc == scope {
			return true
		}
	}
	return false
}
