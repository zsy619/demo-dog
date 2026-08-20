package idx

import (
	"testing"
)

func TestAddAndSearch(t *testing.T) {
	i := New()
	i.Add(&Doc{ID: "1", Fields: map[string]string{"title": "hello world"}})
	i.Add(&Doc{ID: "2", Fields: map[string]string{"title": "world peace"}})
	i.Add(&Doc{ID: "3", Fields: map[string]string{"title": "hello there"}})
	hits := i.Search("hello", 10)
	if len(hits) != 2 {
		t.Fatalf("hits: %d", len(hits))
	}
	if hits[0].DocID != "1" && hits[0].DocID != "3" {
		t.Fatalf("unexpected top: %+v", hits[0])
	}
}

func TestSearch_NoMatch(t *testing.T) {
	i := New()
	i.Add(&Doc{ID: "1", Fields: map[string]string{"title": "foo"}})
	hits := i.Search("bar", 10)
	if len(hits) != 0 {
		t.Fatal("expected no hits")
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	i := New()
	hits := i.Search("", 10)
	if len(hits) != 0 {
		t.Fatal("expected empty")
	}
}

func TestSearch_Limit(t *testing.T) {
	i := New()
	for n := 0; n < 5; n++ {
		i.Add(&Doc{ID: string(rune('a'+n)), Fields: map[string]string{"title": "foo"}})
	}
	hits := i.Search("foo", 2)
	if len(hits) != 2 {
		t.Fatalf("limit: %d", len(hits))
	}
}

func TestSearch_Ranking(t *testing.T) {
	i := New()
	i.Add(&Doc{ID: "1", Fields: map[string]string{"title": "foo foo bar"}})
	i.Add(&Doc{ID: "2", Fields: map[string]string{"title": "foo bar bar bar"}})
	hits := i.Search("foo bar", 10)
	if hits[0].DocID != "2" {
		t.Fatalf("expected 2 first, got %s", hits[0].DocID)
	}
}

func TestDelete(t *testing.T) {
	i := New()
	i.Add(&Doc{ID: "1", Fields: map[string]string{"title": "foo"}})
	i.Delete("1")
	hits := i.Search("foo", 10)
	if len(hits) != 0 {
		t.Fatal("expected no hits")
	}
}

func TestStopwords(t *testing.T) {
	i := New().WithStopwords([]string{"the", "a"})
	i.Add(&Doc{ID: "1", Fields: map[string]string{"title": "the foo"}})
	hits := i.Search("the", 10)
	if len(hits) != 0 {
		t.Fatal("expected stopword filter")
	}
	hits = i.Search("foo", 10)
	if len(hits) != 1 {
		t.Fatal("foo should still match")
	}
}

func TestAdd_NoID(t *testing.T) {
	i := New()
	i.Add(&Doc{Fields: map[string]string{"title": "foo"}})
	if i.Size() != 0 {
		t.Fatal("missing ID should be ignored")
	}
}

func TestMultiField(t *testing.T) {
	i := New()
	i.Add(&Doc{ID: "1", Fields: map[string]string{
		"title": "alpha",
		"body":  "beta",
	}})
	hits := i.Search("beta", 10)
	if len(hits) != 1 || hits[0].DocID != "1" {
		t.Fatal("multi-field")
	}
}

func TestTokenize(t *testing.T) {
	out := tokenize("Hello, WORLD! foo-bar.baz")
	if len(out) != 5 || out[0] != "hello" || out[1] != "world" || out[2] != "foo" || out[3] != "bar" || out[4] != "baz" {
		t.Fatalf("tokenize: %v", out)
	}
}

func TestSize(t *testing.T) {
	i := New()
	if i.Size() != 0 {
		t.Fatal("empty")
	}
	i.Add(&Doc{ID: "a", Fields: map[string]string{"title": "x"}})
	if i.Size() != 1 {
		t.Fatal("size")
	}
}
