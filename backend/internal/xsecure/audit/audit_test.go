package audit

import "testing"

func TestAppend(t *testing.T) {
	l := New()
	l.Append(Event{Actor: "a", Action: "login"})
	if l.Len() != 1 {
		t.Fatal("len")
	}
}

func TestRecent(t *testing.T) {
	l := New()
	for i := 0; i < 5; i++ {
		l.Append(Event{Actor: "x"})
	}
	r := l.Recent(3)
	if len(r) != 3 {
		t.Fatal("recent")
	}
}

func TestFilter(t *testing.T) {
	l := New()
	l.Append(Event{Actor: "alice", Action: "login"})
	l.Append(Event{Actor: "bob", Action: "logout"})
	f := l.Filter("alice", "")
	if len(f) != 1 || f[0].Actor != "alice" {
		t.Fatal("filter actor")
	}
	f = l.Filter("", "logout")
	if len(f) != 1 || f[0].Action != "logout" {
		t.Fatal("filter action")
	}
}

func TestClear(t *testing.T) {
	l := New()
	l.Append(Event{})
	l.Clear()
	if l.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestMarshal(t *testing.T) {
	e := Event{Actor: "a", Action: "x"}
	b, err := e.Marshal()
	if err != nil || len(b) == 0 {
		t.Fatal("marshal")
	}
}
