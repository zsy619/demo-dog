package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/alerts"
)

// LoadAlertRulesFile 解析一个 YAML 或 JSON 格式的告警规则文件。其
// 格式与 alerts.Rule 的 JSON tag 一致。YAML 支持是一个小型
// 手写的子集（顶层 rules 是 "- name:" 条目组成的数组），
// 因为引入真正的 YAML 解析器会增加依赖。
func LoadAlertRulesFile(path string) ([]alerts.Rule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var asArray []alerts.Rule
	if err := json.Unmarshal(b, &asArray); err == nil && len(asArray) > 0 {
		return asArray, nil
	}
	var asObject struct {
		Rules []alerts.Rule `json:"rules"`
	}
	if err := json.Unmarshal(b, &asObject); err == nil && len(asObject.Rules) > 0 {
		return asObject.Rules, nil
	}
	return parseYAMLRules(b)
}

func (s *Server) SetAlertRules(rules []alerts.Rule) {
	if s.alerts == nil {
		return
	}
	s.alerts.eng.SetRules(rules)
}

func (s *Server) RunAlertTicker(interval time.Duration) {
	if s.alerts == nil {
		return
	}
	_, cancel := context.WithCancel(context.Background())
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-time.After(interval):
				s.alerts.eng.Evaluate()
			}
		}
	}()
	_ = cancel
}

func parseYAMLRules(b []byte) ([]alerts.Rule, error) {
	var rules []alerts.Rule
	cur := alerts.Rule{}
	inRules := false
	inItem := false
	for _, ln := range splitLines(b) {
		trimmed := trim(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !inRules {
			if trimmed == "rules:" {
				inRules = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			if inItem {
				rules = append(rules, cur)
			}
			cur = alerts.Rule{}
			inItem = true
			trimmed = trimmed[2:]
		}
		if !inItem {
			continue
		}
		if colon := strings.IndexByte(trimmed, ':'); colon > 0 {
			k := trim(trimmed[:colon])
			v := trim(trimmed[colon+1:])
			applyRuleField(&cur, k, v)
		}
	}
	if inItem {
		rules = append(rules, cur)
	}
	return rules, nil
}

func applyRuleField(r *alerts.Rule, k, v string) {
	switch k {
	case "name":
		r.Name = v
	case "description":
		r.Description = v
	case "service":
		r.Service = v
	case "target":
		r.Target = parseFloat(v)
	case "window":
		r.Window = parseDuration(v)
	case "fast_window":
		r.FastWindow = parseDuration(v)
	case "fast_burn":
		r.FastBurn = parseFloat(v)
	case "slow_burn":
		r.SlowBurn = parseFloat(v)
	case "severity":
		r.Severity = alerts.Severity(v)
	case "channels":
		r.Channels = strings.Split(v, ",")
	}
}

func splitLines(b []byte) []string {
	out := []string{}
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, string(b[start:i]))
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, string(b[start:]))
	}
	return out
}

func trim(s string) string {
	return strings.TrimSpace(s)
}

func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

var _ = http.MethodPost
