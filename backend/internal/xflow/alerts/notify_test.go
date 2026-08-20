package alerts

import (
	"testing"
)

func TestSeverityToPD(t *testing.T) {
	cases := []struct{ in, want string }{
		{"info", "info"},
		{"warn", "warning"},
		{"warning", "warning"},
		{"error", "critical"},
		{"fatal", "critical"},
		{"critical", "critical"},
		{"unknown", "info"},
	}
	for _, c := range cases {
		if got := severityToPD(c.in); got != c.want {
			t.Fatalf("severityToPD(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestBuildMIME(t *testing.T) {
	got := buildMIME("hello", "from@example.com", []string{"to@example.com"}, "body")
	want := "From: from@example.com\r\nTo: to@example.com\r\nSubject: hello\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nbody"
	if got != want {
		t.Fatalf("\ngot:\n%q\nwanted:\n%q", got, want)
	}
}

func TestEmailChannel_Validate(t *testing.T) {
	// Empty host should error.
	e := &EmailChannel{}
	if err := e.Send(nil, NotifyOpts{}); err == nil {
		t.Fatal("expected error for empty host")
	}
	e.Host = "smtp.example.com"
	if err := e.Send(nil, NotifyOpts{}); err == nil {
		t.Fatal("expected error for empty recipients")
	}
}

func TestPagerDutyChannel_Validate(t *testing.T) {
	p := &PagerDutyChannel{}
	if err := p.Send(nil, NotifyOpts{}); err == nil {
		t.Fatal("expected error for empty integration key")
	}
}

func TestWebhookChannel_EmptyURL(t *testing.T) {
	w := &WebhookChannel{}
	if err := w.Send(nil, NotifyOpts{}); err == nil {
		t.Fatal("expected error for empty URL")
	}
}
