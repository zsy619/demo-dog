package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xbilling"
)

// ---- W1.5: 多租户用量计量与计费导出 ----
//
// 通过 s.metering (xbilling.Meter) 提供:
// - POST /api/v1/billing/usage 记录一次用量。
// - GET  /api/v1/billing/usage?tenant=X&period=YYYY-MM 查询。
// - GET  /api/v1/billing/usage.csv?tenant=X&period=YYYY-MM 导出。

// handleUsage 处理 GET/POST /api/v1/billing/usage。
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	if s.metering == nil {
		writeJSON(w, http.StatusOK, map[string]any{"recorded": 0})
		return
	}
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Tenant string    `json:"tenant"`
			Metric string    `json:"metric"`
			Delta  int64     `json:"delta"`
			At     time.Time `json:"at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if body.Tenant == "" || body.Metric == "" {
			writeError(w, http.StatusBadRequest, errors.New("tenant and metric required"))
			return
		}
		if body.Delta == 0 {
			writeError(w, http.StatusBadRequest, errors.New("delta must be non-zero"))
			return
		}
		s.metering.Record(body.Tenant, xbilling.Metric(body.Metric), body.Delta, body.At)
		writeJSON(w, http.StatusOK, map[string]any{
			"tenant": body.Tenant,
			"metric": body.Metric,
			"delta":  body.Delta,
		})
	case http.MethodGet:
		q := r.URL.Query()
		tenant := q.Get("tenant")
		period := q.Get("period")
		metric := q.Get("metric")
		if tenant != "" && period != "" {
			if metric == "" {
				rows := filterByPeriod(s.metering.All(), tenant, period)
				writeJSON(w, http.StatusOK, map[string]any{
					"tenant":  tenant,
					"period":  period,
					"metrics": rows,
				})
				return
			}
			val, ok := s.metering.Query(tenant, metric, period)
			writeJSON(w, http.StatusOK, map[string]any{
				"tenant":  tenant,
				"metric":  metric,
				"period":  period,
				"value":   val,
				"present": ok,
			})
			return
		}
		if tenant != "" {
			usages := s.metering.UsageFor(tenant)
			writeJSON(w, http.StatusOK, map[string]any{
				"tenant": tenant,
				"usage":  usages,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"rows": s.metering.All(),
		})
	default:
		w.Header().Set("Allow", "GET POST")
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET POST only"))
	}
}

// handleUsageCSV 处理 GET /api/v1/billing/usage.csv。
// 返回 text/csv。
func (s *Server) handleUsageCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET only"))
		return
	}
	if s.metering == nil {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		_, _ = w.Write([]byte("period,tenant,metric,value,updated_at\n"))
		return
	}
	rows := s.metering.All()
	q := r.URL.Query()
	if t := q.Get("tenant"); t != "" {
		rows = filterByTenant(rows, t)
	}
	if p := q.Get("period"); p != "" {
		rows = filterByPeriod(rows, "", p)
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=billing-%s.csv", time.Now().UTC().Format("20060102-150405")))
	_, _ = w.Write(xbilling.EncodeCSV(rows))
}

func filterByTenant(rows []xbilling.PeriodTotal, tenant string) []xbilling.PeriodTotal {
	out := make([]xbilling.PeriodTotal, 0, len(rows))
	for _, r := range rows {
		if r.Tenant == tenant {
			out = append(out, r)
		}
	}
	return out
}

func filterByPeriod(rows []xbilling.PeriodTotal, tenant, period string) []xbilling.PeriodTotal {
	out := make([]xbilling.PeriodTotal, 0, len(rows))
	for _, r := range rows {
		if (tenant == "" || r.Tenant == tenant) && (period == "" || r.Period == period) {
			out = append(out, r)
		}
	}
	return out
}
