// Package feed reads and writes the mentions feed, version 1, and the local
// state file that holds triage. docs/feed.md is the spec and schema.json the
// JSON Schema. A collector in Go can use this package to write a feed; any
// other language can write the JSON directly.
package feed

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Version is the feed version this package reads and writes.
const Version = 1

// Feed is one mentions.json file.
type Feed struct {
	Schema    string  `json:"$schema,omitempty"`
	Version   int     `json:"version"`
	Project   Project `json:"project"`
	Generated string  `json:"generated"`
	Groups    []Group `json:"groups,omitempty"`
	Items     []Item  `json:"items"`
}

// Project names the project the mentions are about.
type Project struct {
	Name     string `json:"name"`
	Subtitle string `json:"subtitle,omitempty"`
	URL      string `json:"url,omitempty"`
	Colors   Colors `json:"colors,omitzero"`
}

// Colors are the project's own colours, as #rrggbb.
type Colors struct {
	Accent  string `json:"accent,omitempty"`
	Accent2 string `json:"accent2,omitempty"`
}

// Group is a sidebar entry the feed adds or changes.
type Group struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Section string `json:"section,omitempty"`
	Color   string `json:"color,omitempty"`
	Icon    string `json:"icon,omitempty"`
	ASCII   string `json:"ascii,omitempty"`
}

// Item is one mention.
type Item struct {
	ID        string             `json:"id"`
	Source    string             `json:"source"`
	Kind      string             `json:"kind"`
	Group     string             `json:"group,omitempty"`
	URL       string             `json:"url"`
	Title     string             `json:"title"`
	Author    string             `json:"author,omitempty"`
	Published string             `json:"published,omitempty"`
	Excerpt   string             `json:"excerpt,omitempty"`
	LinkTitle string             `json:"link_title,omitempty"`
	Files     []string           `json:"files,omitempty"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	Tags      []string           `json:"tags,omitempty"`
	Relevance string             `json:"relevance,omitempty"`
	Reason    string             `json:"reason,omitempty"`
	FirstSeen string             `json:"first_seen,omitempty"`
	LastSeen  string             `json:"last_seen,omitempty"`
	State     string             `json:"state,omitempty"`
}

// Relevance values. An empty relevance means RelReal.
const (
	RelReal       = "real"
	RelLikely     = "likely"
	RelUnverified = "unverified"
	RelNoise      = "noise"
	RelAutomation = "automation"
	RelOwn        = "own"
)

// Triage states.
const (
	StateNew      = "new"
	StateSeen     = "seen"
	StateFeatured = "featured"
	StateIgnored  = "ignored"
)

// Hidden reports whether a viewer hides the item until asked.
func (it *Item) Hidden() bool {
	switch it.Relevance {
	case RelNoise, RelAutomation, RelOwn:
		return true
	}
	return false
}

// GroupID is the item's sidebar entry: its own group, or the default for
// its source and kind.
func (it *Item) GroupID() string {
	if it.Group != "" {
		return it.Group
	}
	return DefaultGroup(it.Source, it.Kind)
}

// DefaultGroup is the table in docs/feed.md.
func DefaultGroup(source, kind string) string {
	switch source {
	case "github":
		switch kind {
		case "issue":
			return "issues"
		case "pr":
			return "prs"
		case "discussion":
			return "discussions"
		case "repo":
			return "repos"
		case "dependent":
			return "dependents"
		}
		return "code"
	case "web", "devto":
		switch kind {
		case "newsletter":
			return "newsletters"
		case "directory":
			return "directories"
		}
		return "articles"
	case "registry":
		return "registries"
	case "hn":
		return "hackernews"
	}
	return source
}

// ParseTime reads an RFC 3339 time or a plain date. A bad or empty string
// gives the zero time.
func ParseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Parse decodes and checks a feed.
func Parse(b []byte) (*Feed, error) {
	var f Feed
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("the feed is not valid JSON: %w", err)
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

var (
	reColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	reSlug  = regexp.MustCompile(`^[a-z0-9_-]+$`)
)

// Validate checks the rules a reader depends on. It returns every problem it
// finds, joined.
func (f *Feed) Validate() error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	switch {
	case f.Version == 0:
		bad("the feed has no version")
	case f.Version > Version:
		bad("the feed is version %d and this reader knows version %d only", f.Version, Version)
	}
	if strings.TrimSpace(f.Project.Name) == "" {
		bad("project.name is empty")
	}
	for _, c := range []string{f.Project.Colors.Accent, f.Project.Colors.Accent2} {
		if c != "" && !reColor.MatchString(c) {
			bad("project.colors: %q is not #rrggbb", c)
		}
	}
	if f.Generated != "" && ParseTime(f.Generated).IsZero() {
		bad("generated: %q is not a time", f.Generated)
	}
	for i, g := range f.Groups {
		if g.ID == "" || g.Name == "" {
			bad("groups[%d] needs an id and a name", i)
		}
		if g.Color != "" && !reColor.MatchString(g.Color) {
			bad("groups[%d].color: %q is not #rrggbb", i, g.Color)
		}
	}
	seen := make(map[string]bool, len(f.Items))
	for i := range f.Items {
		it := &f.Items[i]
		where := fmt.Sprintf("items[%d]", i)
		if it.ID == "" {
			bad("%s has no id", where)
		} else if seen[it.ID] {
			bad("%s: the id %q is used twice", where, it.ID)
		}
		seen[it.ID] = true
		if !reSlug.MatchString(it.Source) {
			bad("%s: source %q must be lower case letters, digits, - or _", where, it.Source)
		}
		if !reSlug.MatchString(it.Kind) {
			bad("%s: kind %q must be lower case letters, digits, - or _", where, it.Kind)
		}
		if it.URL == "" {
			bad("%s has no url", where)
		}
		switch it.Relevance {
		case "", RelReal, RelLikely, RelUnverified, RelNoise, RelAutomation, RelOwn:
		default:
			bad("%s: relevance %q is not known", where, it.Relevance)
		}
		switch it.State {
		case "", StateNew, StateSeen, StateFeatured, StateIgnored:
		default:
			bad("%s: state %q is not known", where, it.State)
		}
		for k, v := range it.Metrics {
			if v < 0 {
				bad("%s: metric %s is below zero", where, k)
			}
		}
		if len(errs) > 20 {
			bad("and more")
			break
		}
	}
	return errors.Join(errs...)
}

// Encode writes the feed as indented JSON with a final newline.
func (f *Feed) Encode() ([]byte, error) {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Public returns a copy of the feed with the featured items only, without
// the private fields. featured reports the triage state of an id.
func (f *Feed) Public(featured func(it *Item) bool) *Feed {
	out := &Feed{
		Version:   Version,
		Project:   f.Project,
		Generated: time.Now().UTC().Format(time.RFC3339),
		Groups:    f.Groups,
		Items:     []Item{},
	}
	for i := range f.Items {
		it := f.Items[i]
		if !featured(&it) {
			continue
		}
		it.Group = it.GroupID()
		it.Relevance, it.Reason, it.Files = "", "", nil
		it.FirstSeen, it.LastSeen, it.State = "", "", ""
		out.Items = append(out.Items, it)
	}
	return out
}
