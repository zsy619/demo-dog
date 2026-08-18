package otlp

import "testing"

func TestRedactDefault_Password(t *testing.T) {
	body := `connecting user with password=hunter2 in url`
	out, _ := DefaultRedactor(body, nil)
	if out == body {
		t.Errorf("password not redacted: %q", out)
	}
	if !contains(out, "***REDACTED***") {
		t.Errorf("redaction marker missing: %q", out)
	}
}

func TestRedactDefault_Bearer(t *testing.T) {
	body := `Authorization: Bearer abc.def.ghi`
	out, _ := DefaultRedactor(body, nil)
	if contains(out, "abc.def.ghi") {
		t.Errorf("bearer leaked: %q", out)
	}
}

func TestRedactDefault_AuthHeader(t *testing.T) {
	body := `headers: authorization: Basic dXNlcjpwYXNz`
	out, _ := DefaultRedactor(body, nil)
	if contains(out, "dXNlcjpwYXNz") {
		t.Errorf("auth header leaked: %q", out)
	}
}

func TestRedactDefault_Email(t *testing.T) {
	body := `user alice@example.com logged in`
	out, _ := DefaultRedactor(body, nil)
	if contains(out, "alice@example.com") {
		t.Errorf("email leaked: %q", out)
	}
}

func TestRedactDefault_AttrKeys(t *testing.T) {
	attrs := map[string]string{
		"password":  "hunter2",
		"user":      "alice",
		"x-api-key": "key-123",
	}
	_, out := DefaultRedactor("login attempt", attrs)
	if out["password"] == "hunter2" {
		t.Errorf("password attribute not redacted")
	}
	if out["x-api-key"] == "key-123" {
		t.Errorf("x-api-key not redacted")
	}
	if out["user"] != "alice" {
		t.Errorf("non-sensitive attribute was modified")
	}
}

func TestRedactDefault_NoMatchPassesThrough(t *testing.T) {
	body := `harmless log line`
	out, _ := DefaultRedactor(body, nil)
	if out != body {
		t.Errorf("expected pass-through, got %q", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
