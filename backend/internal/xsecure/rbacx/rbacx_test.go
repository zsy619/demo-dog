package rbacx

import (
	"testing"
)

func TestAllowed(t *testing.T) {
	e := New()
	e.GrantRole("admin", "users:read", "users:write")
	e.Bind("alice", "admin")
	if !e.Allowed("alice", "users:read") {
		t.Fatal("应允许")
	}
}

func TestAllowed_Deny(t *testing.T) {
	e := New()
	e.GrantRole("user", "users:read")
	e.Bind("alice", "user")
	if e.Allowed("alice", "users:write") {
		t.Fatal("应拒绝")
	}
}

func TestUnbind(t *testing.T) {
	e := New()
	e.GrantRole("user", "users:read")
	e.Bind("alice", "user")
	e.Unbind("alice", "user")
	if e.Allowed("alice", "users:read") {
		t.Fatal("应拒绝")
	}
}

func TestHasRole(t *testing.T) {
	e := New()
	e.GrantRole("admin", "x")
	e.Bind("alice", "admin")
	if !e.HasRole("alice", "admin") {
		t.Fatal("hasrole")
	}
}

func TestRoles(t *testing.T) {
	e := New()
	e.Bind("a", "admin")
	e.Bind("a", "user")
	if len(e.Roles("a")) != 2 {
		t.Fatal("roles")
	}
}

func TestRolePerms(t *testing.T) {
	e := New()
	e.GrantRole("admin", "a", "b")
	if len(e.RolePerms("admin")) != 2 {
		t.Fatal("perms")
	}
}

func TestReset(t *testing.T) {
	e := New()
	e.GrantRole("admin", "x")
	e.Bind("a", "admin")
	e.Reset()
	if e.HasRole("a", "admin") {
		t.Fatal("reset")
	}
}
