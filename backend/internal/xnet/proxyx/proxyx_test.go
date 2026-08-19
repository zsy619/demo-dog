package proxyx

import "testing"

func TestParseBasic(t *testing.T) {
	p, err := Parse("http://example.com:8080")
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme != "http" || p.Host != "example.com" || p.Port != 8080 {
		t.Fatal("p", p)
	}
}

func TestParseUser(t *testing.T) {
	p, err := Parse("http://alice:secret@example.com:8080")
	if err != nil {
		t.Fatal(err)
	}
	if p.User != "alice" || p.Pass != "secret" {
		t.Fatal("user", p)
	}
}

func TestParseNoPort(t *testing.T) {
	p, err := Parse("socks5://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p.Host != "example.com" {
		t.Fatal("host", p)
	}
}

func TestParseBad(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Fatal("empty")
	}
	if _, err := Parse("noscheme"); err == nil {
		t.Fatal("no scheme")
	}
}

func TestString(t *testing.T) {
	p := Proxy{Scheme: "http", Host: "a.com", Port: 80, User: "u", Pass: "p"}
	s := p.String()
	if s != "http://u:p@a.com:80" {
		t.Fatal("s", s)
	}
}

func TestIsZero(t *testing.T) {
	if !(Proxy{}).IsZero() {
		t.Fatal("zero")
	}
	if (Proxy{Host: "x"}).IsZero() {
		t.Fatal("not zero")
	}
}
