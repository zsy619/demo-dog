// Package transform normalises OTel-shaped values to the simpler wire
// format the DOG collector expects. The backend has its own Normalize
// step but doing some of the work in the SDK lets the wire payload stay
// smaller and reduces server-side CPU.
//
// Functions in this package return plain strings so the package does not
// depend on the otlp package and there is no risk of an import cycle.
package transform

import (
	"strings"
	"time"
)

// NormalizeSeverity maps an arbitrary user-provided severity string to the
// constant string the backend understands. Unknown values fall back to
// INFO so they still appear in the live tail.
func NormalizeSeverity(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return "TRACE"
	case "DEBUG":
		return "DEBUG"
	case "INFO", "":
		return "INFO"
	case "WARN", "WARNING":
		return "WARN"
	case "ERROR", "ERR":
		return "ERROR"
	case "FATAL", "PANIC", "EMERG", "ALERT", "CRITICAL", "CRIT":
		return "FATAL"
	}
	return "INFO"
}

// NormalizeStatus maps an arbitrary span status string to the collector
// vocabulary: ok / error / unset. Anything unrecognised becomes unset
// which is the OTel default.
func NormalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ok", "success", "succeeded":
		return "ok"
	case "error", "err", "failed", "failure":
		return "error"
	}
	return "unset"
}

// NormalizeMetricType maps an arbitrary metric type to
// gauge/counter/histogram.
func NormalizeMetricType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "counter", "sum", "monotonic":
		return "counter"
	case "histogram", "hist", "distribution":
		return "histogram"
	}
	return "gauge"
}

// Now returns the current local time. Wrapped in a var for tests.
var Now = func() time.Time { return time.Now() }
