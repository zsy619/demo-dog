package routex

import "testing"

func TestLongestMatch(t *testing.T) {
	tab := New()
	tab.Add("/api", "a")
	tab.Add("/api/v1", "b")
	tab.Add("/api/v1/users", "c")
	tab.Add("/api/v2", "d")
	tests := []struct {
		p    string
		want string
	}{
		{"/api/v1/users/123", "c"},
		{"/api/v1/foo", "b"},
		{"/api/v2/bar", "d"},
		{"/api/foo", "a"},
		{"/unknown", ""},
	}
	for _, tt := range tests {
		got, _ := tab.Match(tt.p)
		if got != tt.want {
			t.Fatalf("Match(%s)=%q want %q", tt.p, got, tt.want)
		}
	}
}
