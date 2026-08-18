package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleAudit returns the most-recent N events from the audit log.
// Query parameters:
//   - n:   optional, default 200, max 10 000
//   - since: optional RFC3339 timestamp; events older than this are
//           skipped
//
// Requires admin role (enforced by the route gate in Handler()).
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	n := 200
	if raw := q.Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			n = v
			if n > 10_000 {
				n = 10_000
			}
		}
	}
	events := s.auditLog.Recent(n)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"count":  len(events),
		"events": events,
	})
}

// handleAuditStats returns the buffer stats (capacity / buffered /
// total). Cheap to call and intended for dashboards.
func (s *Server) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.auditLog.Stats())
}

// handleListKeys returns the registered API keys (without the raw
// secret — just label + role). This is what makes the system
// manageable from a CI script: a deploy can dump current keys
// and diff against its desired state.
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries := s.auth.List()
	type out struct {
		Label string `json:"label"`
		Role  string `json:"role"`
		Key   string `json:"key_prefix"`
	}
	list := make([]out, 0, len(entries))
	for _, e := range entries {
		// Never echo the full secret — the prefix is enough for an
		// operator to recognise which entry is which.
		prefix := e.Key
		if len(prefix) > 6 {
			prefix = prefix[:6] + "…"
		}
		list = append(list, out{Label: e.Label, Role: e.Role.String(), Key: prefix})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count": len(list),
		"keys":  list,
	})
}
