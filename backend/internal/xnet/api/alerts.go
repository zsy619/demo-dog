package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
)

type alertsEngine struct {
	mu    sync.Mutex
	eng   *alerts.Engine
	slos  []*alerts.SLO
}

func newAlertsEngine(s *store.Doris) *alertsEngine {
	provider := &storeProvider{s: s}
	return &alertsEngine{eng: alerts.NewEngine(provider)}
}

// AddSLO appends an SLO definition.
func (a *alertsEngine) AddSLO(s *alerts.SLO) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.slos = append(a.slos, s)
}

// SLOStatus computes a fresh budget status for every registered SLO.
func (a *alertsEngine) SLOStatus(now time.Time) []alerts.BudgetStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]alerts.BudgetStatus, 0, len(a.slos))
	for _, s := range a.slos {
		if err := s.Validate(); err != nil {
			continue
		}
		st, _ := alerts.Compute(s, zeroCountSink{}, now)
		out = append(out, st)
	}
	return out
}

// zeroCountSink is a placeholder sink that reports no traffic.
type zeroCountSink struct{}

func (zeroCountSink) Counter(name string, window time.Duration) int64 { return 0 }

type storeProvider struct {
	s *store.Doris
}

func (p *storeProvider) SuccessRatio(service string, window time.Duration) (float64, int) {
	since := time.Now().Add(-window).UnixMilli()
	if service == "" {
		service = "*"
	}
	ok, err := p.s.SuccessCounts(service, since)
	total := ok + err
	if total == 0 {
		return 1.0, 0
	}
	return float64(ok) / float64(total), total
}

func (s *Server) handleAlertsRules(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil {
		writeJSON(w, http.StatusOK, map[string]any{"rules": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rules": s.alerts.sortedRules(),
	})
}

func (s *Server) handleAlertsFires(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil {
		writeJSON(w, http.StatusOK, map[string]any{"fires": []any{}})
		return
	}
	n := 100
	if v := r.URL.Query().Get("n"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			n = parsed
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fires": s.alerts.eng.Recent(n),
	})
}

func (a *alertsEngine) sortedRules() []alerts.Rule {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.eng.SortedRules()
}

func (a *alertsEngine) getRule(name string) (alerts.Rule, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.eng.GetRule(name)
}

func (a *alertsEngine) upsertRule(r alerts.Rule) (alerts.Rule, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.eng.UpsertRule(r)
}

func (a *alertsEngine) deleteRule(name string) (alerts.Rule, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.eng.DeleteRule(name)
}
