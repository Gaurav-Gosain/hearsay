//go:build ignore

// Command gen writes ../sample.json, the made-up feed behind --demo. Every
// name, handle, title and link in it is invented. Links point at example.com,
// example.org and example.net. Run it from internal/demo:
//
//	go run gen/main.go
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Gaurav-Gosain/hearsay/feed"
)

// anchor is "now" in the sample. demo.Feed moves every date so the anchor
// lands on the real now.
var anchor = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

var rng = rand.New(rand.NewPCG(7, 11))

func pick[T any](xs []T) T { return xs[rng.IntN(len(xs))] }

var handles = []string{
	"pixelfern", "quietkernel", "mossbyte", "tidepool_dev", "nightjar", "copperleaf",
	"saltmarsh", "lowtide", "brambleworks", "inkwell_io", "fogline", "kitefield",
	"emberwake", "cindertrail", "duskhollow", "ravenmoss", "stillwater", "hollowpine",
	"glintwire", "parchment_cat", "marrowlight", "thistle_dev", "orchardloop", "sablefin",
}

var channels = []string{
	"Terminal Tuesdays", "The Quiet Shell", "Dotfile Diaries", "Byte Garden",
	"Code by Candlelight", "Small Tools Weekly", "Tiling Corner", "Shell Shock Studio",
}

var outlets = []string{
	"The Plain Terminal", "Small Tools Review", "Command Line Notes", "The Tiling Desk",
	"Open Shelf", "Pane & Prompt",
}

var repos = []string{
	"pixelfern/dotfiles", "quietkernel/config", "mossbyte/nix-config", "tidepool/awesome-terminal",
	"nightjar/homelab", "copperleaf/dotfiles", "inkwell/weekly", "fogline/awesome-tui",
	"kitefield/setup", "emberwake/.config", "saltmarsh/lumen-themes", "lowtide/lumen.nvim",
	"brambleworks/workstation", "glintwire/tui-list", "thistle/arch-setup", "sablefin/dots",
}

var videoTitles = []string{
	"lumen in 100 seconds", "I replaced my status bar with lumen", "A calm terminal dashboard: lumen",
	"lumen first look: widgets, themes and a clock", "Five terminal tools I use every day (lumen is #1)",
	"Building a lumen widget from scratch", "lumen vs. three other dashboards", "Ricing my terminal with lumen",
	"lumen 1.0 is out: what changed", "Live: setting up lumen on a fresh laptop",
}

var postTexts = []string{
	"lumen is the first terminal dashboard that does not get in my way",
	"Spent the evening theming lumen. Worth it.",
	"Anyone else running lumen on a Raspberry Pi? Works great here",
	"lumen 1.0 just landed and the clock widget alone sells it",
	"Switched from my old status script to lumen, never going back",
	"Is there a way to make lumen show my calendar? asking for a friend",
	"New blog post: how I use lumen to watch my builds",
	"lumen + a tiling WM = a very quiet desk",
	"The lumen config format is so pleasant to write",
	"Tried lumen today. The defaults are good enough that I barely changed anything",
	"Wrote a lumen widget for my weather station",
	"lumen just saved me from missing a failing deploy",
	"Bookmarking lumen for the weekend",
	"Is lumen worth it over a plain shell prompt? Thoughts?",
	"Shipping a lumen theme pack this week",
	"Three weeks with lumen on my work laptop. Short review in the thread",
	"Finally a dashboard that respects my colour scheme",
	"lumen's keyboard map is the best part, everything is one key away",
	"Ported my conky setup to lumen in an evening",
	"The lumen docs are short and that is a compliment",
	"lumen idles at almost no CPU, which matters on this old machine",
	"Today I learned lumen can read a feed of my CI runs",
	"Put lumen on the wall monitor in the office. People keep asking about it",
	"Small thing, but the lumen clock font is lovely",
	"Any lumen users on FreeBSD? Curious how it runs there",
	"My lumen config is now 12 lines long and does everything I need",
	"lumen made me delete four shell scripts",
	"Recorded a short clip of my lumen layout, link below",
	"lumen and a cup of tea: my morning standup view",
	"Opened my first PR to lumen today, the maintainers were kind",
}

var articleTitles = []string{
	"A quiet desk: living in lumen for a month", "lumen and the case for small tools",
	"Five dashboards for your terminal, ranked", "How lumen draws a 60 fps clock in a terminal",
	"Getting started with lumen on macOS", "Notes on lumen's widget model",
}

var issueTitles = []string{
	"Add a lumen widget to the status line", "Support lumen themes in the installer",
	"Document how to run under lumen", "Crash when lumen resizes quickly", "Package lumen for this distro",
	"Integrate with lumen's widget protocol", "Add lumen to the list of supported dashboards",
}

var prTitles = []string{
	"chore: update lumen to 1.0.2", "feat: add lumen config", "docs: mention lumen in the README",
	"Add lumen to awesome list", "nix: add lumen module", "lumen: 0.9.1 -> 1.0.0",
}

var nouns = []string{"widget", "theme", "clock", "layout", "status line", "plugin", "config"}

func id(url string) string {
	h := sha1.Sum([]byte(url))
	return hex.EncodeToString(h[:])[:16]
}

// ago returns a time before the anchor, skewed to recent weeks so the
// histogram climbs towards now.
func ago() time.Time {
	d := rng.ExpFloat64() * 45 * 24
	if d > 300*24 {
		d = rng.Float64() * 300 * 24
	}
	return anchor.Add(-time.Duration(d * float64(time.Hour)))
}

func date(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func n(lo, hi int) float64 { return float64(lo + rng.IntN(hi-lo+1)) }

func main() {
	f := &feed.Feed{
		Version: feed.Version,
		Project: feed.Project{
			Name: "lumen", Subtitle: "in the wild", URL: "https://example.com/lumen",
			Colors: feed.Colors{Accent: "#7c6cff", Accent2: "#f08bd0"},
		},
		Generated: date(anchor),
	}
	add := func(it feed.Item) {
		if it.ID == "" {
			it.ID = id(it.URL)
		}
		f.Items = append(f.Items, it)
	}
	seq := 0
	next := func() int { seq++; return seq }

	for _, t := range videoTitles {
		u := fmt.Sprintf("https://example.com/video/%04d", next())
		views := n(80, 9000)
		if t == "lumen in 100 seconds" {
			views = 48210
		}
		add(feed.Item{Source: "youtube", Kind: "video", URL: u, Title: t, Author: pick(channels),
			Published: date(ago()), Excerpt: "A walk through " + pick(nouns) + "s, themes and the keys you need on day one.",
			Metrics: map[string]float64{"views": views, "likes": float64(int(views / n(25, 60))), "comments": n(0, 120)}})
	}
	socials := []struct{ src, kind string }{
		{"bluesky", "post"}, {"mastodon", "post"}, {"reddit", "post"}, {"reddit", "comment"},
		{"hackernews", "comment"}, {"lemmy", "post"}, {"x", "post"}, {"x", "reply"}, {"stackexchange", "question"},
	}
	texts := append([]string(nil), postTexts...)
	rng.Shuffle(len(texts), func(i, j int) { texts[i], texts[j] = texts[j], texts[i] })
	for i := 0; i < 48; i++ {
		s := socials[rng.IntN(len(socials))]
		text := texts[i%len(texts)]
		u := fmt.Sprintf("https://example.org/%s/%s/%05d", s.src, s.kind, next())
		author := pick(handles)
		if s.src == "x" || s.src == "bluesky" || s.src == "mastodon" {
			author = "@" + author
		}
		m := map[string]float64{}
		switch s.src {
		case "reddit", "hackernews", "lemmy":
			m["points"] = n(1, 220)
			m["comments"] = n(0, 60)
		case "stackexchange":
			m["score"] = n(0, 12)
			m["views"] = n(20, 900)
		case "x":
			m["views"] = n(30, 12000)
			m["likes"] = n(0, 140)
			m["reposts"] = n(0, 20)
			m["replies"] = n(0, 12)
		default:
			m["likes"] = n(0, 90)
			m["reposts"] = n(0, 15)
			m["replies"] = n(0, 8)
		}
		rel := pick([]string{"real", "real", "real", "likely", "unverified"})
		reason := map[string]string{"real": "links the project page", "likely": "names lumen with terminal words",
			"unverified": "says lumen without context"}[rel]
		title := text
		if s.kind == "question" {
			title = "How do I make a lumen " + pick(nouns) + " update every second?"
		}
		add(feed.Item{Source: s.src, Kind: s.kind, URL: u, Title: title, Author: author, Published: date(ago()),
			Excerpt: text + ". " + pick([]string{"Highly recommend.", "Screenshots in the thread.", "Config below.", ""}),
			Metrics: m, Relevance: rel, Reason: reason})
	}
	for _, t := range articleTitles {
		u := fmt.Sprintf("https://example.net/articles/%03d", next())
		add(feed.Item{Source: "web", Kind: "article", URL: u, Title: t, Author: pick(outlets), Published: date(ago()),
			Excerpt: "A long read about " + pick(nouns) + "s and why small tools win.", Reason: "links the project page"})
	}
	add(feed.Item{Source: "web", Kind: "newsletter", URL: "https://example.net/newsletter/issue-42", Title: "Small Tools Weekly #42",
		Author: "Small Tools Weekly", Published: date(ago()), Excerpt: "This week: lumen 1.0, a new tiling layout, and more."})
	add(feed.Item{Source: "web", Kind: "directory", URL: "https://example.net/directory/lumen", Title: "lumen on The Tool Shelf",
		Author: "The Tool Shelf", Published: date(ago())})
	type gh struct{ repo, kind, title string }
	var pairs []gh
	for _, r := range repos {
		for _, t := range issueTitles {
			pairs = append(pairs, gh{r, pick([]string{"issue", "issue", "discussion"}), t})
		}
		for _, t := range prTitles {
			pairs = append(pairs, gh{r, "pr", t})
		}
	}
	rng.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
	usedTitle := map[string]int{}
	taken := 0
	for _, p := range pairs {
		if taken == 30 || usedTitle[p.title] >= 2 {
			continue
		}
		usedTitle[p.title]++
		taken++
		repo, k, t := p.repo, p.kind, p.repo+": "+p.title
		u := fmt.Sprintf("https://example.com/%s/%s/%d", repo, k, next())
		rel := pick([]string{"real", "likely", "likely"})
		it := feed.Item{Source: "github", Kind: k, URL: u, Title: t, Author: strings.Split(repo, "/")[0],
			Published: date(ago()), Metrics: map[string]float64{"comments": n(0, 14), "reactions": n(0, 9)}, Relevance: rel}
		if k == "pr" && rng.IntN(5) == 0 {
			it.Relevance, it.Reason, it.Author = "automation", "a bot bumps a version", "update-bot"
		}
		add(it)
	}
	for _, r := range [][2]string{
		{"saltmarsh/lumen-themes", "Themes for lumen"}, {"lowtide/lumen.nvim", "lumen inside your editor"},
		{"glintwire/lumen-widgets", "A pack of lumen widgets"}, {"kitefield/lumen-weather", "Weather for lumen"},
	} {
		repo := r[0]
		u := "https://example.com/" + repo
		add(feed.Item{Source: "github", Kind: "repo", URL: u, Title: repo + ": " + r[1],
			Author: strings.Split(repo, "/")[0], Published: date(ago()), Metrics: map[string]float64{"stars": n(2, 340), "forks": n(0, 30)}})
	}
	for _, repo := range []string{"pixelfern/dotfiles", "mossbyte/nix-config", "emberwake/.config", "sablefin/dots", "thistle/arch-setup"} {
		files := []string{".config/lumen/config.toml"}
		if rng.IntN(2) == 0 {
			files = append(files, ".config/lumen/widgets/clock.toml", "install.sh")
		}
		add(feed.Item{ID: id("code:dotfiles:" + repo), Source: "github", Kind: "code", Group: "dotfiles",
			URL: "https://example.com/" + repo, Title: repo, Author: strings.Split(repo, "/")[0],
			Published: date(ago()), Files: files, Relevance: "likely"})
	}
	for _, repo := range []string{"tidepool/awesome-terminal", "fogline/awesome-tui", "inkwell/weekly"} {
		add(feed.Item{ID: id("code:lists:" + repo), Source: "github", Kind: "code", Group: "lists",
			URL: "https://example.com/" + repo + "/README.md", Title: repo, Author: strings.Split(repo, "/")[0],
			Published: date(ago()), Files: []string{"README.md"}})
	}
	add(feed.Item{Source: "registry", Kind: "package", URL: "https://example.org/packages/lumen", Title: "lumen on the package index",
		Author: "package index", Published: date(ago()), Metrics: map[string]float64{"installs_30d": 2140, "installs_90d": 5302}})
	// A few hidden ones, so h has something to show.
	for i := 0; i < 6; i++ {
		u := fmt.Sprintf("https://example.org/noise/%d", next())
		add(feed.Item{Source: pick([]string{"reddit", "x", "bluesky"}), Kind: "post", URL: u,
			Title:  pick([]string{"lumen of the room was perfect", "the lumen count on this bulb is wrong", "lumen means light, right?"}),
			Author: pick(handles), Published: date(ago()), Relevance: "noise", Reason: "the word lumen, not the project"})
	}
	// A few featured, so the featured view has something to show.
	for _, i := range []int{0, 2, 12} {
		f.Items[i].State = feed.StateFeatured
	}
	sort.SliceStable(f.Items, func(i, j int) bool { return f.Items[i].Published > f.Items[j].Published })
	if err := f.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, err := f.Encode()
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("sample.json", b, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote sample.json: %d items\n", len(f.Items))
}
