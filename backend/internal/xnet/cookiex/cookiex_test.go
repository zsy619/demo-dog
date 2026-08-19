package cookiex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParse(t *testing.T) {
	m := Parse("a=1; b=2; c=3")
	if m["a"] != "1" || m["b"] != "2" || m["c"] != "3" {
		t.Fatal("parse", m)
	}
}

func TestParse_Empty(t *testing.T) {
	m := Parse("")
	if len(m) != 0 {
		t.Fatal("empty", m)
	}
}

func TestSerialize(t *testing.T) {
	s := Serialize(map[string]string{"a": "1", "b": "2"})
	if s == "" {
		t.Fatal("serialize")
	}
}

func TestGetSetDelete(t *testing.T) {
	w := httptest.NewRecorder()
	SetCookie(w, "sid", "abc", 60, "/", "", false, true)
	SetCookie(w, "theme", "dark", 60, "/", "", false, false)
	DeleteCookie(w, "sid", "/")
	// 验证 header 设置成功
	if w.Result().Header.Get("Set-Cookie") == "" {
		t.Fatal("set")
	}
}

func TestMustParse(t *testing.T) {
	c, err := MustParse("foo=bar;Path=/;HttpOnly")
	if err != nil || c.Name != "foo" {
		t.Fatal("parse", c)
	}
}

func TestGetCookie(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "a", Value: "1"})
	v, ok := GetCookie(r, "a")
	if !ok || v != "1" {
		t.Fatal("get", v)
	}
	if _, ok := GetCookie(r, "missing"); ok {
		t.Fatal("miss")
	}
}
