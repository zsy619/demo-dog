package sqlb

import (
	"errors"
	"testing"
)

func newSchema() *Schema {
	s := NewSchema()
	s.Register("users", "id", "name", "email")
	s.Register("users", "uid") // 实际属于 users
	return s
}

func TestSelect(t *testing.T) {
	b := New(newSchema())
	b.Select("users", "id", "name")
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
	want := "SELECT id, name FROM users"
	if b.String() != want {
		t.Fatal("got:", b.String())
	}
}

func TestSelect_Star(t *testing.T) {
	b := New(newSchema())
	b.Select("users", "*")
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
}

func TestSelect_BadTable(t *testing.T) {
	b := New(newSchema())
	b.Select("ghost", "id")
	if b.Err() == nil {
		t.Fatal("应报错")
	}
}

func TestSelect_BadColumn(t *testing.T) {
	b := New(newSchema())
	b.Select("users", "password")
	if b.Err() == nil {
		t.Fatal("应报错")
	}
}

func TestWhere(t *testing.T) {
	b := New(newSchema())
	b.Select("users", "*").Where("=", "id", 1)
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
	if got := b.String(); got != "SELECT * FROM users WHERE id = ?" {
		t.Fatal(got)
	}
	if len(b.Args()) != 1 || b.Args()[0] != 1 {
		t.Fatal("args")
	}
}

func TestAndWhere(t *testing.T) {
	b := New(newSchema())
	b.Select("users", "*").Where("=", "id", 1).AndWhere("=", "name", "a")
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
	want := "SELECT * FROM users WHERE id = ? AND name = ?"
	if got := b.String(); got != want {
		t.Fatal(got)
	}
}

func TestLimit(t *testing.T) {
	b := New(newSchema())
	b.Select("users", "*").Limit(10)
	if got := b.String(); got != "SELECT * FROM users LIMIT 10" {
		t.Fatal(got)
	}
}

func TestIdent_Invalid(t *testing.T) {
	if err := Ident("drop;--"); !errors.Is(err, ErrBadIdent) {
		t.Fatal("应拒绝")
	}
	if err := Ident(""); !errors.Is(err, ErrBadIdent) {
		t.Fatal("应拒绝空")
	}
}
