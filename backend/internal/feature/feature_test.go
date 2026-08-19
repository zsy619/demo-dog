package feature

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(0).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestFlag_Validate(t *testing.T) {
	cases := []struct {
		name string
		f    Flag
		err  bool
	}{
		{"valid bool", Flag{Name: "x", Kind: KindBool, Default: true}, false},
		{"valid string", Flag{Name: "x", Kind: KindString, Default: "y"}, false},
		{"valid int", Flag{Name: "x", Kind: KindInt, Default: 1}, false},
		{"empty name", Flag{Kind: KindBool, Default: true}, true},
		{"bool wrong", Flag{Name: "x", Kind: KindBool, Default: "y"}, true},
		{"string wrong", Flag{Name: "x", Kind: KindString, Default: 1}, true},
		{"int wrong", Flag{Name: "x", Kind: KindInt, Default: "y"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.f.Validate()
			if (err != nil) != c.err {
				t.Fatalf("err=%v want %v", err, c.err)
			}
		})
	}
}

func TestManager_Register(t *testing.T) {
	m := newTestManager()
	if err := m.Register(&Flag{Name: "x", Kind: KindBool, Default: false}); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(&Flag{Name: "x"}); err == nil {
		t.Fatal("dup")
	}
}

func TestManager_MustRegister_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: true})
	m.MustRegister(&Flag{Name: "x"})
}

func TestManager_GetAndList(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "b", Kind: KindBool, Default: false})
	m.MustRegister(&Flag{Name: "a", Kind: KindBool, Default: true})
	list := m.List()
	if list[0].Name != "a" || list[1].Name != "b" {
		t.Fatal("sort")
	}
	if _, ok := m.Get("a"); !ok {
		t.Fatal("get")
	}
}

func TestEvaluate_Default(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: true})
	v, ok := m.Evaluate("x", "acme")
	if !ok || v != true {
		t.Fatal("default")
	}
}

func TestEvaluate_Override(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: true})
	if err := m.SetOverride("x", "acme", false, "alice"); err != nil {
		t.Fatal(err)
	}
	v, _ := m.Evaluate("x", "acme")
	if v != false {
		t.Fatal("override not applied")
	}
	v, _ = m.Evaluate("x", "other")
	if v != true {
		t.Fatal("other tenant should see default")
	}
}

func TestEvaluate_Missing(t *testing.T) {
	m := newTestManager()
	if _, ok := m.Evaluate("missing", "acme"); ok {
		t.Fatal("expected miss")
	}
}

func TestTypedAccessors(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "b", Kind: KindBool, Default: true})
	m.MustRegister(&Flag{Name: "s", Kind: KindString, Default: "hello"})
	m.MustRegister(&Flag{Name: "i", Kind: KindInt, Default: 7})
	if v, _ := m.Bool("b", "x"); !v {
		t.Fatal("bool")
	}
	if v, _ := m.String("s", "x"); v != "hello" {
		t.Fatal("string")
	}
	if v, _ := m.Int("i", "x"); v != 7 {
		t.Fatal("int")
	}
	if _, ok := m.Bool("missing", "x"); ok {
		t.Fatal("missing bool")
	}
	if _, ok := m.String("missing", "x"); ok {
		t.Fatal("missing string")
	}
	if _, ok := m.Int("missing", "x"); ok {
		t.Fatal("missing int")
	}
}

func TestSetOverride_WrongKind(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: false})
	if err := m.SetOverride("x", "a", "not-bool", "actor"); err == nil {
		t.Fatal("expected kind mismatch")
	}
}

func TestSetOverride_UnknownFlag(t *testing.T) {
	m := newTestManager()
	if err := m.SetOverride("missing", "a", true, "actor"); err == nil {
		t.Fatal("expected unknown flag")
	}
}

func TestClearOverride(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: true})
	m.SetOverride("x", "a", false, "act")
	if err := m.ClearOverride("x", "a", "act"); err != nil {
		t.Fatal(err)
	}
	v, _ := m.Evaluate("x", "a")
	if v != true {
		t.Fatal("clear should revert to default")
	}
	if err := m.ClearOverride("x", "a", "act"); err != nil {
		t.Fatal("second clear should be no-op")
	}
}

func TestClearOverride_Unknown(t *testing.T) {
	m := newTestManager()
	if err := m.ClearOverride("missing", "a", "act"); err == nil {
		t.Fatal("expected unknown flag")
	}
}

func TestOverrides(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: false})
	m.SetOverride("x", "b", true, "a")
	m.SetOverride("x", "a", true, "a")
	over := m.Overrides("x")
	if len(over) != 2 || over[0].Tenant != "a" {
		t.Fatalf("overrides: %+v", over)
	}
}

func TestAudit(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: false})
	m.SetOverride("x", "a", true, "alice")
	m.SetOverride("x", "a", false, "alice")
	m.ClearOverride("x", "a", "alice")
	audit := m.Audit()
	if len(audit) != 3 {
		t.Fatalf("audit: %d", len(audit))
	}
	if audit[0].Action != "set" || audit[1].Action != "set" || audit[2].Action != "clear" {
		t.Fatalf("actions: %+v", audit)
	}
	if audit[0].OldValue != nil {
		t.Fatal("first old should be nil (no prior override)")
	}
	if audit[0].NewValue != true {
		t.Fatal("first new should be true")
	}
}

func TestAuditFor(t *testing.T) {
	m := newTestManager()
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: false})
	m.SetOverride("x", "a", true, "alice")
	m.SetOverride("x", "b", true, "bob")
	got := m.AuditFor("a")
	if len(got) != 1 || got[0].Tenant != "a" {
		t.Fatal("filter")
	}
}

func TestAuditCap(t *testing.T) {
	m := NewManager(3).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
	m.MustRegister(&Flag{Name: "x", Kind: KindBool, Default: false})
	for i := 0; i < 10; i++ {
		m.SetOverride("x", "a", i%2 == 0, "act")
	}
	audit := m.Audit()
	if len(audit) != 3 {
		t.Fatalf("audit cap: %d", len(audit))
	}
}
