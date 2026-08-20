package api

import (
	"testing"
)

func TestOTLPSeverityText_FromText(t *testing.T) {
	if got := severityText(0, "FOO"); got != "FOO" {
		t.Fatalf("got %q", got)
	}
}

func TestOTLPSeverityText_FromNumber(t *testing.T) {
	cases := []struct{ n int; want string }{
		{5, "DEBUG"},
		{9, "INFO"},
		{13, "WARN"},
		{17, "ERROR"},
		{21, "FATAL"},
		{1, "TRACE"},
		{0, "INFO"}, // unknown -> default
	}
	for _, c := range cases {
		if got := severityText(c.n, ""); got != c.want {
			t.Fatalf("n=%d got=%q want=%q", c.n, got, c.want)
		}
	}
}

func TestHexFromBytes(t *testing.T) {
	if got := hexFromBytes([]byte{0xde, 0xad, 0xbe, 0xef}); got != "deadbeef" {
		t.Fatalf("got %q", got)
	}
	if got := hexFromBytes(nil); got != "" {
		t.Fatalf("expected empty for nil, got %q", got)
	}
}

func TestFloatToString(t *testing.T) {
	if got := floatToString(3.5, 2); got != "3.5" {
		t.Fatalf("got %q", got)
	}
	if got := floatToString(-1.25, 4); got != "-1.25" {
		t.Fatalf("got %q", got)
	}
	if got := floatToString(0, 6); got != "0" {
		t.Fatalf("got %q", got)
	}
}

func TestItoa(t *testing.T) {
	cases := []struct{ n int64; want string }{
		{0, "0"},
		{42, "42"},
		{-7, "-7"},
		{9223372036854775807, "9223372036854775807"},
	}
	for _, c := range cases {
		if got := itoa(c.n); got != c.want {
			t.Fatalf("n=%d got=%q want=%q", c.n, got, c.want)
		}
	}
}

func TestOtlpAttrsToMap(t *testing.T) {
	attrs := []otlpKeyValue{
		{Key: "region", Value: otlpAnyValue{StringValue: "us-east-1"}},
	}
	m := otlpAttrsToMap(attrs)
	if m["region"] != "us-east-1" {
		t.Fatalf("got %v", m)
	}
}

func TestAnyValueToString(t *testing.T) {
	tf := true; ff := false; one := int64(1); pi := 3.14
	cases := []struct{
		v otlpAnyValue
		want string
	}{
		{otlpAnyValue{StringValue: "hi"}, "hi"},
		{otlpAnyValue{BoolValue: &tf}, "true"},
		{otlpAnyValue{BoolValue: &ff}, "false"},
		{otlpAnyValue{IntValue: &one}, "1"},
		{otlpAnyValue{DoubleValue: &pi}, "3.140000"},
	}
	for _, c := range cases {
		if got := anyValueToString(c.v); got != c.want {
			t.Fatalf("got %q want %q", got, c.want)
		}
	}
}
