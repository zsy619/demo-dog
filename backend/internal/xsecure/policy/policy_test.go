package policy

import (
	"testing"
)

func TestEq(t *testing.T) {
	p := New()
	n, err := p.Parse(`role == "admin"`)
	if err != nil {
		t.Fatal(err)
	}
	if !Eval(n, map[string]any{"role": "admin"}) {
		t.Fatal("应为真")
	}
	if Eval(n, map[string]any{"role": "user"}) {
		t.Fatal("应为假")
	}
}

func TestIn(t *testing.T) {
	p := New()
	n, _ := p.Parse(`region in ["cn", "us"]`)
	if !Eval(n, map[string]any{"region": "cn"}) {
		t.Fatal("应命中")
	}
	if Eval(n, map[string]any{"region": "jp"}) {
		t.Fatal("应不命中")
	}
}

func TestAndOrNot(t *testing.T) {
	p := New()
	tests := []struct {
		expr string
		ctx  map[string]any
		want bool
	}{
		{`role == "admin" AND region == "cn"`, map[string]any{"role": "admin", "region": "cn"}, true},
		{`role == "admin" AND region == "us"`, map[string]any{"role": "admin", "region": "cn"}, false},
		{`role == "admin" OR role == "root"`, map[string]any{"role": "root"}, true},
		{`NOT role == "admin"`, map[string]any{"role": "user"}, true},
	}
	for _, c := range tests {
		n, err := p.Parse(c.expr)
		if err != nil {
			t.Fatal(err)
		}
		if got := Eval(n, c.ctx); got != c.want {
			t.Fatalf("%s => %v want %v", c.expr, got, c.want)
		}
	}
}

func TestParen(t *testing.T) {
	p := New()
	n, _ := p.Parse(`(role == "admin" OR role == "root") AND active == 1`)
	if !Eval(n, map[string]any{"role": "root", "active": 1}) {
		t.Fatal("应为真")
	}
	if Eval(n, map[string]any{"role": "root", "active": 0}) {
		t.Fatal("应为假")
	}
}

func TestMissingField(t *testing.T) {
	p := New()
	n, _ := p.Parse(`role == "admin"`)
	if Eval(n, map[string]any{}) {
		t.Fatal("缺失字段应为假")
	}
}

func TestParseError(t *testing.T) {
	p := New()
	if _, err := p.Parse(`role = "x"`); err == nil {
		t.Fatal("期望错误")
	}
}

func TestNilNode(t *testing.T) {
	if !Eval(nil, nil) {
		t.Fatal("nil 应永真")
	}
}
