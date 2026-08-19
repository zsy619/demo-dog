package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/alerts"
)

// handleRules returns the live SLO burn-rate rules in the shape
// Prometheus /api/v1/rules returns. Compatible with the Prometheus
// UI Rules tab and any tooling that talks to that endpoint.
//
// Query parameters (all optional):
//   type=alert   only alert rules (we only have alert rules)
//   state=...    informational, returned in the response shape
//   group=<name> ignored (we group per-rule for simplicity)
func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	if s.alerts == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"data":   map[string]any{"groups": []any{}},
		})
		return
	}
	rules := s.alerts.sortedRules()
	type ruleState struct {
		State       string    `json:"state"`
		LastError   string    `json:"lastError,omitempty"`
		EvaluatedAt time.Time `json:"evaluatedAt"`
		Health      string    `json:"health"`
	}
	type ruleEntry struct {
		Name        string            `json:"name"`
		Query       string            `json:"query"`
		Duration    float64           `json:"duration"` // seconds
		Labels      map[string]string `json:"labels,omitempty"`
		Annotations map[string]string `json:"annotations,omitempty"`
		Type        string            `json:"type"`
		Health      string            `json:"health"`
		State       string            `json:"state"`
		LastEvaluation ruleState      `json:"lastEvaluation"`
	}
	type ruleGroup struct {
		Name  string      `json:"name"`
		File  string      `json:"file"`
		Rules []ruleEntry `json:"rules"`
	}

	groups := make([]ruleGroup, 0, len(rules))
	for _, rl := range rules {
		groups = append(groups, ruleGroup{
			Name: rl.Name,
			File: "slo.burnrate",
			Rules: []ruleEntry{{
				Name:     rl.Name,
				Query:    promQueryFor(rl),
				Duration: rl.Window.Seconds(),
				Labels: map[string]string{
					"severity": string(rl.Severity),
					"service":  rl.Service,
				},
				Annotations: map[string]string{
					"description": rl.Description,
					"summary":     rl.Description,
					"fast_burn":   fmt.Sprintf("%g", rl.FastBurn),
					"slow_burn":   fmt.Sprintf("%g", rl.SlowBurn),
					"target":      fmt.Sprintf("%g", rl.Target),
				},
				Type:  "alerting",
				Health: "ok",
				State:  ruleStateFor(rl),
				LastEvaluation: ruleState{
					State:       ruleStateFor(rl),
					Health:      "ok",
					EvaluatedAt: time.Now(),
				},
			}},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "success",
		"data": map[string]any{
			"groups": groups,
		},
	})
}

// promQueryFor synthesises a Prometheus-shaped query string for a
// burn-rate rule. The string is informative, not evaluable against
// real PromQL — but it tells humans what the rule watches.
func promQueryFor(r alerts.Rule) string {
	svc := r.Service
	if svc == "" {
		svc = "*"
	}
	return fmt.Sprintf("burn_rate(service=%q, window=%s, fast_burn=%g, slow_burn=%g, target=%g)",
		svc, r.Window, r.FastBurn, r.SlowBurn, r.Target)
}

// ruleStateFor returns "inactive" by default. We don't track per-rule
// firing state in the engine yet; the rules endpoint exposes the
// configuration, the fires endpoint exposes the history.
func ruleStateFor(_ alerts.Rule) string {
	return "inactive"
}
