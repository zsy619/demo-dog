package transform

import "testing"

func TestNormalizeSeverity(t *testing.T) {
	cases := map[string]string{
		"trace":       "TRACE",
		"DEBUG":       "DEBUG",
		"":            "INFO",
		"info":        "INFO",
		"warn":        "WARN",
		"WARNING":     "WARN",
		"error":       "ERROR",
		"ERR":         "ERROR",
		"fatal":       "FATAL",
		"panic":       "FATAL",
		"emerg":       "FATAL",
		"alert":       "FATAL",
		"critical":    "FATAL",
		"crit":        "FATAL",
		"unknown-xyz": "INFO",
	}
	for in, want := range cases {
		got := NormalizeSeverity(in)
		if got != want {
			t.Errorf("NormalizeSeverity(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"ok":         "ok",
		"OK":         "ok",
		"success":    "ok",
		"err":        "error",
		"error":      "error",
		"failed":     "error",
		"FAILURE":    "error",
		"":           "unset",
		"garbage":    "unset",
	}
	for in, want := range cases {
		got := NormalizeStatus(in)
		if got != want {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeMetricType(t *testing.T) {
	cases := map[string]string{
		"counter":      "counter",
		"Counter":      "counter",
		"sum":          "counter",
		"monotonic":    "counter",
		"histogram":    "histogram",
		"hist":         "histogram",
		"distribution": "histogram",
		"gauge":        "gauge",
		"":             "gauge",
	}
	for in, want := range cases {
		got := NormalizeMetricType(in)
		if got != want {
			t.Errorf("NormalizeMetricType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHex(t *testing.T) {
	if len(Hex(8)) != 16 {
		t.Fatal("Hex(8) should be 16 hex chars")
	}
	if len(Hex(16)) != 32 {
		t.Fatal("Hex(16) should be 32 hex chars")
	}
}
