package ui

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"example.com/mentions-ui/feed"
)

// Mention is one row of the list.
type Mention struct {
	ID         string
	URL        string
	Source     string
	Type       string
	Title      string
	Author     string
	T          time.Time
	Snippet    string
	LinkTitle  string
	Verdict    string // "" when the feed does not say
	Reason     string
	Engagement map[string]float64
	Group      string
	Files      []string
	Initial    string // the feed's own state, if any

	Score     float64
	Badge     string
	BadgeTier int // 0 quiet, 1 warm, 2 hot

	titleRunes []rune
	hayLower   []rune // lower(title) + " " + lower(author) + " " + group name
	snipLower  string
}

// Dataset is everything the view shows.
type Dataset struct {
	Visible []*Mention // real, likely, unverified
	Hidden  []*Mention // noise, automation, own
	Project feed.Project
	Updated time.Time
	Sources int // groups with at least one item
}

// NewDataset turns a feed into rows. It also sets up the sidebar groups and
// the colours, so call it once, before the model is made.
func NewDataset(f *feed.Feed) *Dataset {
	ds := &Dataset{Project: f.Project, Updated: feed.ParseTime(f.Generated)}
	used := map[string]bool{}
	for i := range f.Items {
		it := &f.Items[i]
		m := fromItem(it)
		used[m.Group] = true
		if it.Hidden() {
			ds.Hidden = append(ds.Hidden, m)
		} else {
			ds.Visible = append(ds.Visible, m)
		}
	}
	setGroups(f.Groups, used)
	ds.Sources = len(used)
	for _, m := range ds.Visible {
		finish(m)
	}
	for _, m := range ds.Hidden {
		finish(m)
	}
	return ds
}

var (
	reWS       = regexp.MustCompile(`\s+`)
	reEmbedURI = regexp.MustCompile(`"uri":\s*"([^"]+)"`)
	reEmbedTtl = regexp.MustCompile(`"title":\s*"([^"]+)`)
)

func fromItem(it *feed.Item) *Mention {
	m := &Mention{
		ID: it.ID, URL: it.URL, Source: it.Source, Type: it.Kind,
		Title: it.Title, Author: it.Author, Snippet: it.Excerpt, LinkTitle: it.LinkTitle,
		Verdict: it.Relevance, Reason: it.Reason, Files: it.Files,
		Engagement: map[string]float64{}, Group: it.GroupID(), Initial: it.State,
		T: feed.ParseTime(it.Published),
	}
	for k, v := range it.Metrics {
		if v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			m.Engagement[k] = v
		}
	}
	// Some collectors leave a raw embed record at the end of a post. Keep
	// the text, and lift the linked page's title out of it.
	if i := strings.Index(m.Snippet, ` {"`); i >= 0 {
		rest := m.Snippet[i:]
		if m.LinkTitle == "" {
			if mm := reEmbedTtl.FindStringSubmatch(rest); mm != nil {
				m.LinkTitle = strings.TrimSpace(mm[1])
			} else if mm := reEmbedURI.FindStringSubmatch(rest); mm != nil {
				m.LinkTitle = mm[1]
			}
		}
		m.Snippet = strings.TrimSpace(m.Snippet[:i])
	}
	if len(m.Files) > 0 {
		m.Engagement["files"] = float64(len(m.Files))
		if m.Snippet == "" {
			m.Snippet = "Matching files in " + m.Title + ":"
		}
	}
	return m
}

func finish(m *Mention) {
	m.Title = cleanText(m.Title)
	m.Author = cleanText(m.Author)
	if m.Title == "" {
		m.Title = m.URL
	}
	m.Score = engagementScore(m.Engagement)
	m.Badge, m.BadgeTier = badgeFor(m)
	m.titleRunes = []rune(m.Title)
	hay := make([]rune, 0, len(m.titleRunes)+40)
	for _, r := range m.titleRunes {
		hay = append(hay, unicode.ToLower(r))
	}
	hay = append(hay, ' ')
	for _, r := range m.Author {
		hay = append(hay, unicode.ToLower(r))
	}
	hay = append(hay, ' ')
	if g := groupByKey(m.Group); g != nil {
		for _, r := range g.Name {
			hay = append(hay, unicode.ToLower(r))
		}
	}
	m.hayLower = hay
	m.snipLower = strings.ToLower(m.Snippet + " " + strings.Join(m.Files, " ") + " " + m.URL)
}

// cleanText flattens newlines and drops runes whose terminal width is not
// settled (emoji presentation selectors, joiners), so columns stay aligned.
func cleanText(s string) string {
	s = reWS.ReplaceAllString(s, " ")
	var b strings.Builder
	for _, r := range s {
		if isUnsettled(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func isUnsettled(r rune) bool {
	switch {
	case r == 0x200d, r == 0xfe0f, r == 0xfe0e, r == 0x20e3:
		return true
	case r >= 0x1f000 && r <= 0x1faff: // emoji blocks
		return true
	case r >= 0x2600 && r <= 0x27bf: // misc symbols, dingbats
		return true
	case r >= 0x1f1e6 && r <= 0x1f1ff:
		return true
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r), unicode.Is(unicode.Cf, r), unicode.IsControl(r):
		return true
	}
	return false
}

var scoreWeights = map[string]float64{
	"views": 0.01, "likes": 1, "stars": 3, "forks": 2, "reactions": 1, "comments": 2,
	"replies": 2, "reposts": 3, "score": 1, "points": 1, "upvotes": 1, "votes": 1,
	"boosts": 3, "installs_30d": 0.05, "downloads": 0.01, "popularity": 0.5, "bookmarks": 1, "files": 0.02,
}

func engagementScore(e map[string]float64) float64 {
	var s float64
	for k, v := range e {
		s += v * scoreWeights[k]
	}
	return s
}

// metricOrder lists metrics from the most telling to the least.
var metricOrder = []struct{ key, label string }{
	{"views", "views"}, {"installs_30d", "dl/30d"}, {"downloads", "downloads"}, {"stars", "stars"},
	{"likes", "likes"}, {"reactions", "reacts"}, {"score", "points"}, {"points", "points"},
	{"upvotes", "upvotes"}, {"votes", "votes"}, {"reposts", "reposts"}, {"boosts", "boosts"},
	{"comments", "comments"}, {"replies", "replies"}, {"files", "files"},
}

func badgeFor(m *Mention) (string, int) {
	for _, mo := range metricOrder {
		v, ok := m.Engagement[mo.key]
		if !ok || v <= 0 {
			continue
		}
		label := mo.label
		if v == 1 && strings.HasSuffix(label, "s") {
			label = strings.TrimSuffix(label, "s")
		}
		tier := 0
		switch mo.key {
		case "views", "installs_30d", "downloads":
			if v >= 10000 {
				tier = 2
			} else if v >= 1000 {
				tier = 1
			}
		case "files":
		default:
			if v >= 100 {
				tier = 2
			} else if v >= 10 {
				tier = 1
			}
		}
		return humanCount(v) + " " + label, tier
	}
	return "", 0
}

func humanCount(v float64) string {
	switch {
	case v >= 1e6:
		return trimZero(fmt.Sprintf("%.1f", v/1e6)) + "M"
	case v >= 1e3:
		return trimZero(fmt.Sprintf("%.1f", v/1e3)) + "k"
	}
	return strconv.Itoa(int(math.Round(v)))
}

func trimZero(s string) string { return strings.TrimSuffix(s, ".0") }

func commaInt(n int) string {
	if n < 0 {
		return "-" + commaInt(-n)
	}
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
