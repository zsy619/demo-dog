package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/alerts"
	"github.com/zsy619/demo-dog/backend/internal/store"
)

type alertsEngine struct {
	mu  sync.Mutex
	eng *alerts.Engine
}

func newAlertsEngine(s *store.Doris) *alertsEngine {
	provider := &storeProvider{s: s}
	return &alertsEngine{eng: alerts.NewEngine(provider)}
}

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
