package feed

import (
	"strings"
	"testing"
)

func TestPublicDropsPrivateFields(t *testing.T) {
	f := &Feed{
		Version: Version, Project: Project{Name: "x"}, Generated: "2026-01-01",
		Items: []Item{
			{ID: "a", Source: "web", Kind: "article", URL: "https://example.com/a", Title: "A",
				Relevance: RelLikely, Reason: "why", Files: []string{"f"}, FirstSeen: "2026-01-01", LastSeen: "2026-01-02"},
			{ID: "b", Source: "x", Kind: "post", URL: "https://example.com/b", Title: "B"},
		},
	}
	pub := f.Public(func(it *Item) bool { return it.ID == "a" })
	if len(pub.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(pub.Items))
	}
	b, err := pub.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"relevance", "reason", "files", "first_seen", "last_seen", "state", `"b"`} {
		if strings.Contains(string(b), key) {
			t.Errorf("the public feed still has %s:\n%s", key, b)
		}
	}
	if pub.Items[0].Group != "articles" {
		t.Errorf("group = %q, want the default group written out", pub.Items[0].Group)
	}
}

func TestParseStateReadsTheFlatForm(t *testing.T) {
	s, err := ParseState([]byte(`{"a": "featured", "b": "ignore"}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Get("a") != StateFeatured || s.Get("b") != StateIgnored {
		t.Errorf("got %+v", s.Items)
	}
}

func TestValidateRejectsANewerVersion(t *testing.T) {
	_, err := Parse([]byte(`{"version": 2, "project": {"name": "x"}, "generated": "2026-01-01", "items": []}`))
	if err == nil {
		t.Fatal("a version 2 feed parsed")
	}
}
