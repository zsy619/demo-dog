package cidr

import (
	"net"
	"testing"
)

func TestContains(t *testing.T) {
	ok, err := Contains("10.0.0.0/8", net.ParseIP("10.1.2.3"))
	if err != nil || !ok {
		t.Fatal("contains")
	}
}

func TestContainsNo(t *testing.T) {
	ok, _ := Contains("10.0.0.0/8", net.ParseIP("192.168.1.1"))
	if ok {
		t.Fatal("no")
	}
}

func TestInvalidCIDR(t *testing.T) {
	if _, err := Contains("bad", net.ParseIP("10.0.0.1")); err == nil {
		t.Fatal("invalid")
	}
}

func TestEqual(t *testing.T) {
	if !Equal(net.ParseIP("1.1.1.1"), net.ParseIP("1.1.1.1")) {
		t.Fatal("eq")
	}
	if Equal(net.ParseIP("1.1.1.1"), net.ParseIP("1.1.1.2")) {
		t.Fatal("neq")
	}
	if !Equal(nil, nil) {
		t.Fatal("nil")
	}
}

func TestUint(t *testing.T) {
	v, ok := ToUint32(net.ParseIP("1.2.3.4"))
	if !ok {
		t.Fatal("u32")
	}
	if v == 0 {
		t.Fatal("v")
	}
	ip := FromUint32(v)
	if ip.String() != "1.2.3.4" {
		t.Fatal("round", ip)
	}
}

func TestUint6(t *testing.T) {
	if _, ok := ToUint32(net.ParseIP("::1")); ok {
		t.Fatal("v6")
	}
}

func TestValidate(t *testing.T) {
	if !Validate("1.1.1.1") {
		t.Fatal("ip")
	}
	if !Validate("10.0.0.0/8") {
		t.Fatal("cidr")
	}
	if Validate("bad") {
		t.Fatal("bad")
	}
}
