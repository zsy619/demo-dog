package inspect

import (
	"strings"
	"testing"
)

type Inner struct {
	Value int `json:"value"`
}

type Sample struct {
	Name  string `json:"name"`
	Inner Inner  `json:"inner"`
	List  []int  `json:"list"`
}

func TestOf(t *testing.T) {
	s := Of(Sample{})
	if s.Fields != 3 {
		t.Fatal("fields")
	}
	if s.Depth < 2 {
		t.Fatal("depth")
	}
}

func TestFields(t *testing.T) {
	f := Fields(Sample{}, 4)
	paths := map[string]bool{}
	for _, x := range f {
		paths[x.Path] = true
	}
	for _, want := range []string{"Name", "Inner", "List", "Inner.Value"} {
		if !paths[want] {
			t.Fatal("缺路径:", want)
		}
	}
}

func TestFields_Tags(t *testing.T) {
	f := Fields(Sample{}, 4)
	for _, x := range f {
		if x.Path == "Name" && !strings.Contains(x.Tag, "json") {
			t.Fatal("应有 tag")
		}
	}
}

func TestString(t *testing.T) {
	if !strings.Contains(String(Sample{}), "type=Sample") {
		t.Fatal("string")
	}
}

func TestNilValue(t *testing.T) {
	if Of(nil).Kind == "" {
		t.Fatal("nil")
	}
}
GOEOF
export PATH=/opt/homebrew/bin:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin:$PATH && go test -race -count=1 ./internal/xtool/inspect/ 2>&1 | tail
