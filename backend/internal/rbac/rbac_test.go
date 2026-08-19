package rbac

import (
	"errors"
	"testing"
)

func TestRegister(t *testing.T) {
	m := New()
	if err := m.Register(&Role{Name: "viewer", Permissions: []string{"read"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Get("viewer"); !ok {
		t.Fatal("get")
	}
	if err := m.Register(&Role{Name: "viewer"}); !errors.Is(err, ErrRoleExists) {
		t.Fatal("expected ErrRoleExists")
	}
}

func TestRegister_UnknownParentFails(t *testing.T) {
	m := New()
	err := m.Register(&Role{Name: "x", Parents: []string{"missing"}})
	if !errors.Is(err, ErrRoleMissing) {
		t.Fatal(err)
	}
}

func TestAssignAndPermission(t *testing.T) {
	m := New()
	m.Register(&Role{Name: "viewer", Permissions: []string{"read"}})
	if err := m.Assign("t1", "alice", "viewer"); err != nil {
		t.Fatal(err)
	}
	if !m.Permission("t1", "alice", "read") {
		t.Fatal("should have read")
	}
	if m.Permission("t1", "alice", "write") {
		t.Fatal("should not have write")
	}
}

func TestAssign_UnknownRole(t *testing.T) {
	m := New()
	if err := m.Assign("t1", "alice", "missing"); !errors.Is(err, ErrRoleMissing) {
		t.Fatal(err)
	}
}

func TestRoleInheritance(t *testing.T) {
	m := New()
	m.MustRegister(&Role{Name: "viewer", Permissions: []string{"read"}})
	m.MustRegister(&Role{Name: "editor", Permissions: []string{"write"}, Parents: []string{"viewer"}})
	if err := m.Assign("t1", "alice", "editor"); err != nil {
		t.Fatal(err)
	}
	if !m.Permission("t1", "alice", "read") {
		t.Fatal("should inherit read")
	}
	if !m.Permission("t1", "alice", "write") {
		t.Fatal("should have write")
	}
}

func TestRoleInheritance_Deep(t *testing.T) {
	m := New()
	m.MustRegister(&Role{Name: "a", Permissions: []string{"p1"}})
	m.MustRegister(&Role{Name: "b", Parents: []string{"a"}})
	m.MustRegister(&Role{Name: "c", Parents: []string{"b"}})
	m.MustRegister(&Role{Name: "d", Parents: []string{"c"}})
	if err := m.Assign("t1", "u", "d"); err != nil {
		t.Fatal(err)
	}
	if !m.Permission("t1", "u", "p1") {
		t.Fatal("deep inherit")
	}
}

func TestPermission_TenantIsolation(t *testing.T) {
	m := New()
	m.MustRegister(&Role{Name: "viewer", Permissions: []string{"read"}})
	m.Assign("t1", "alice", "viewer")
	if m.Permission("t2", "alice", "read") {
		t.Fatal("tenant isolation")
	}
}

func TestPermission_NoAssignments(t *testing.T) {
	m := New()
	if m.Permission("t1", "alice", "read") {
		t.Fatal("no assignments should not allow")
	}
}

func TestUnassign(t *testing.T) {
	m := New()
	m.MustRegister(&Role{Name: "viewer", Permissions: []string{"read"}})
	m.Assign("t1", "alice", "viewer")
	m.Unassign("t1", "alice", "viewer")
	if m.Permission("t1", "alice", "read") {
		t.Fatal("should not have read")
	}
}

func TestRoles(t *testing.T) {
	m := New()
	m.MustRegister(&Role{Name: "viewer", Permissions: []string{"read"}})
	m.MustRegister(&Role{Name: "editor", Permissions: []string{"write"}})
	m.Assign("t1", "alice", "viewer")
	m.Assign("t1", "alice", "editor")
	roles := m.Roles("t1", "alice")
	if len(roles) != 2 {
		t.Fatalf("roles: %v", roles)
	}
}

func TestRoles_Missing(t *testing.T) {
	m := New()
	if roles := m.Roles("t1", "alice"); len(roles) != 0 {
		t.Fatal("missing")
	}
}

func TestCycleIsIgnored(t *testing.T) {
	m := New()
	m.MustRegister(&Role{Name: "a", Permissions: []string{"p1"}})
	m.MustRegister(&Role{Name: "b", Permissions: []string{"p2"}, Parents: []string{"a"}})
	// Manual cycle via Assign: not registered, but a cycle
	// through manual assignment does not break the visited map.
	m.MustRegister(&Role{Name: "c", Parents: []string{"a"}})
	m.Assign("t1", "alice", "b")
	m.Assign("t1", "alice", "c")
	if !m.Permission("t1", "alice", "p1") {
		t.Fatal("p1")
	}
}
