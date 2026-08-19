package abac

import (
	"testing"
)

func TestCheck_Allow(t *testing.T) {
	e := New()
	e.Add(Policy{
		Name:     "admin",
		Actions:  []string{"read", "write"},
		Subjects: []string{"alice"},
		Effect:   EffectAllow,
	})
	req := Request{
		Subject: Subject{ID: "alice"},
		Action:  "read",
	}
	if !e.Allowed(req) {
		t.Fatal("应允许")
	}
}

func TestCheck_Deny(t *testing.T) {
	e := New()
	e.Add(Policy{
		Name:     "admin",
		Actions:  []string{"delete"},
		Subjects: []string{"*"},
		Effect:   EffectDeny,
	})
	if e.Allowed(Request{Subject: Subject{ID: "bob"}, Action: "delete"}) {
		t.Fatal("应拒绝")
	}
}

func TestCheck_NoMatch(t *testing.T) {
	e := New()
	e.Add(Policy{Name: "a", Actions: []string{"x"}, Subjects: []string{"a"}, Effect: EffectAllow})
	if e.Allowed(Request{Subject: Subject{ID: "b"}, Action: "y"}) {
		t.Fatal("无匹配应拒")
	}
}

func TestCheck_WithFn(t *testing.T) {
	e := New()
	e.Add(Policy{
		Name: "tag", Actions: []string{"read"}, Subjects: []string{"*"},
		Effect: EffectAllow,
		Fn: func(req Request) bool {
			return req.Resource.Tags["level"].(int) < 3
		},
	})
	ok := e.Allowed(Request{
		Subject:  Subject{ID: "alice"},
		Action:   "read",
		Resource: Resource{Tags: map[string]any{"level": 1}},
	})
	if !ok {
		t.Fatal("应允许")
	}
}

func TestPolicyCount(t *testing.T) {
	e := New()
	if e.PolicyCount() != 0 {
		t.Fatal("empty")
	}
	e.Add(Policy{})
	if e.PolicyCount() != 1 {
		t.Fatal("count")
	}
}

func TestDecisionString(t *testing.T) {
	if DecisionAllow.String() != "allow" {
		t.Fatal("str")
	}
}
