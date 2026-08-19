package headers

import (
	"net/http"
	"testing"
)

func TestSetGet(t *testing.T) {
	h := http.Header{}
	Set(h, Authorization, "Bearer x")
	if GetFirst(h, Authorization) != "Bearer x" {
		t.Fatal("set")
	}
}

func TestAdd(t *testing.T) {
	h := http.Header{}
	Add(h, Accept, "application/json")
	Add(h, Accept, "text/plain")
	if len(h[Accept]) != 2 {
		t.Fatal("add")
	}
}

func TestContentTypeOf(t *testing.T) {
	if ContentTypeOf("json") != "application/json" {
		t.Fatal("json")
	}
	if ContentTypeOf("unknown") != "application/octet-stream" {
		t.Fatal("def")
	}
}

func TestGetFirst_Trim(t *testing.T) {
	h := http.Header{}
	h.Set("X-Test", "  value  ")
	if GetFirst(h, "X-Test") != "value" {
		t.Fatal("trim")
	}
}
