package mimex

import "testing"

func TestFromExt(t *testing.T) {
	if FromExt(".json", "x") != "application/json" {
		t.Fatal("json")
	}
}

func TestFromExt_Default(t *testing.T) {
	if FromExt(".unknown", "application/octet-stream") != "application/octet-stream" {
		t.Fatal("def")
	}
}

func TestFromFile(t *testing.T) {
	if FromFile("foo.html", "x") != "text/html" {
		t.Fatal("file")
	}
}

func TestRegister(t *testing.T) {
	Register(".xfoo", "application/x-foo")
	if FromExt(".xfoo", "") != "application/x-foo" {
		t.Fatal("reg")
	}
}
