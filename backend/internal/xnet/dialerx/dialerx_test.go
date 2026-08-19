package dialerx

import "testing"

func TestDialer(t *testing.T) {
	d := Dialer(Config{Timeout: 1000000000})
	if d.Timeout != 1000000000 {
		t.Fatal("timeout")
	}
}

func TestDialerLocalAddr(t *testing.T) {
	d := Dialer(Config{Timeout: 1000000000, LocalAddr: "127.0.0.1:0"})
	if d.LocalAddr == nil {
		t.Fatal("local addr")
	}
}

func TestSplitHostPort(t *testing.T) {
	h, p, _ := splitHostPort("127.0.0.1:8080")
	if h != "127.0.0.1" || p != 8080 {
		t.Fatal("split", h, p)
	}
}

func TestSplitNoPort(t *testing.T) {
	h, p, _ := splitHostPort("host")
	if h != "host" || p != 0 {
		t.Fatal("no port", h, p)
	}
}

func TestHasPort(t *testing.T) {
	if !hasPort("a:1") || hasPort("a") {
		t.Fatal("hasport")
	}
}
