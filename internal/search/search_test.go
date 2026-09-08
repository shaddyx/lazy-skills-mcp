package search

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"goroutineLeak detection", []string{"goroutine", "leak", "detection"}},
		{"Hello, World!", []string{"hello", "world"}},
		{"a b c", nil},              // all single-char, filtered out
		{"html2", []string{"html"}}, // digit boundary splits, but "2" is len<2 and filtered
		{"", nil},
	}
	for _, tc := range cases {
		got := Tokenize(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Tokenize(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func docs() []*Doc {
	return []*Doc{
		{ID: "dev/golang/testing", Name: "testing", Desc: "table-driven tests and fuzzing", Body: "Use goleak to detect goroutine leaks."},
		{ID: "dev/golang/error-handling", Name: "error-handling", Desc: "wrapping with custom types", Body: "wrap with percent w"},
		{ID: "dev/web/responsive-design", Name: "responsive-design", Desc: "css grid and flexbox", Body: "adaptive layouts across devices"},
	}
}

func TestSearchRanksByFieldWeight(t *testing.T) {
	e := Build(docs())
	// name match (weight 3.0) outranks body-only match
	hits := e.Search("testing", "", 10, true)
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if hits[0].Path != "dev/golang/testing" {
		t.Errorf("top hit = %q, want dev/golang/testing", hits[0].Path)
	}
}

func TestSearchSubtreeRestriction(t *testing.T) {
	e := Build(docs())
	hits := e.Search("golang", "dev/golang", 10, true)
	for _, h := range hits {
		if !within(h.Path, "dev/golang") {
			t.Errorf("hit %q outside subtree", h.Path)
		}
	}
}

func TestSearchBodyExclusion(t *testing.T) {
	e := Build(docs())
	// "goroutine" only appears in the body of testing.
	withBody := e.Search("goroutine", "", 10, true)
	noBody := e.Search("goroutine", "", 10, false)
	if len(withBody) == 0 {
		t.Error("expected body-only hit when includeBody true")
	}
	if len(noBody) != 0 {
		t.Errorf("expected no hits when includeBody false, got %d", len(noBody))
	}
}

func TestSearchZeroHits(t *testing.T) {
	e := Build(docs())
	hits := e.Search("zzznothing", "", 10, true)
	if len(hits) != 0 {
		t.Errorf("expected zero hits, got %d", len(hits))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	e := Build(docs())
	if hits := e.Search("", "", 10, true); len(hits) != 0 {
		t.Errorf("empty query should yield no hits, got %d", len(hits))
	}
}

func TestSearchLimit(t *testing.T) {
	e := Build(docs())
	// "responsive testing" matches two distinct names.
	all := e.Search("responsive testing", "", 10, true)
	hits := e.Search("responsive testing", "", 1, true)
	if len(all) != 2 {
		t.Errorf("expected 2 matches, got %d", len(all))
	}
	if len(hits) != 1 {
		t.Errorf("limit 1 exceeded: got %d", len(hits))
	}
}
