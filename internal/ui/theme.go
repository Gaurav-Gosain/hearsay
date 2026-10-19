package ui

import (
	"sort"
	"strings"

	"github.com/Gaurav-Gosain/hearsay/feed"
)

// RGB is a 24-bit colour, 0xRRGGBB. Bubble Tea lowers it to 256 or 16
// colours on terminals that cannot show truecolor.
type RGB uint32

const noColor RGB = 0xff000000

const (
	cInk      RGB = 0x15141c
	cInkDeep  RGB = 0x111018
	cPanel    RGB = 0x19181f
	cCard     RGB = 0x1c1b26
	cLine     RGB = 0x2b2939
	cLineSoft RGB = 0x211f2b
	cText     RGB = 0xebe8f7
	cSub      RGB = 0xb0abc8
	cMuted    RGB = 0x7d7896
	cFaint    RGB = 0x4f4b63
	cGreen    RGB = 0x5fd8a3
	cAmber    RGB = 0xf3b75c
	cRed      RGB = 0xff6b81
	cBlue     RGB = 0x74b4ff
	cFeatBG   RGB = 0x2a2216
)

// The accent family is a var: a feed or a flag can change it.
var (
	cAccent   RGB = 0x7c6cff
	cAccentHi RGB = 0xa89cff
	cAccentLo RGB = 0x342d6b
	cPink     RGB = 0xf08bd0
	cSel      RGB = 0x2c2656
)

// SetColors changes the accent family. The defaults are what the wordmark
// was drawn for. The other tones are mixed from the accent the same way the
// default ones relate to it.
func SetColors(accent, accent2 string) {
	if c, ok := parseHex(accent); ok {
		cAccent = c
		cAccentHi = lerp(c, 0xffffff, 0.35)
		cAccentLo = lerp(c, cInk, 0.68)
		cSel = lerp(c, cInk, 0.80)
	}
	if c, ok := parseHex(accent2); ok {
		cPink = c
	}
}

func parseHex(s string) (RGB, bool) {
	if len(s) != 7 || s[0] != '#' {
		return 0, false
	}
	var v uint32
	for _, ch := range s[1:] {
		v <<= 4
		switch {
		case ch >= '0' && ch <= '9':
			v |= uint32(ch - '0')
		case ch >= 'a' && ch <= 'f':
			v |= uint32(ch-'a') + 10
		case ch >= 'A' && ch <= 'F':
			v |= uint32(ch-'A') + 10
		default:
			return 0, false
		}
	}
	return RGB(v), true
}

func (c RGB) rgb() (uint8, uint8, uint8) { return uint8(c >> 16), uint8(c >> 8), uint8(c) }

func lerp(a, b RGB, t float64) RGB {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	ar, ag, ab := a.rgb()
	br, bg, bb := b.rgb()
	m := func(x, y uint8) uint32 { return uint32(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return RGB(m(ar, br)<<16 | m(ag, bg)<<8 | m(ab, bb))
}

// Group is one entry of the sources sidebar.
type Group struct {
	Key, Name, Section string
	Glyph, ASCII       string
	Color              RGB
}

// builtinGroups are the sidebar entries a feed gets without asking. The
// colour of "all" follows the accent, so it is set in setGroups.
var builtinGroups = []Group{
	{"youtube", "YouTube", "Social & video", "\uf16a", "Y", 0xff5f5f},
	{"x", "X", "Social & video", "\U000f0b1f", "X", 0xe8e6f3},
	{"bluesky", "Bluesky", "Social & video", "\U000f1589", "B", 0x4f9dff},
	{"reddit", "Reddit", "Social & video", "\uf281", "R", 0xff7a45},
	{"hackernews", "Hacker News", "Social & video", "\uf2e5", "H", 0xff8c1a},
	{"lobsters", "Lobsters", "Social & video", "\U000f0b13", "L", 0xd2553f},
	{"lemmy", "Lemmy", "Social & video", "\U000f1327", "l", 0x22c79a},
	{"mastodon", "Mastodon", "Social & video", "\U000f0ad1", "M", 0x8c8dff},
	{"stackexchange", "Stack Exchange", "Social & video", "\uf18d", "S", 0xf48024},

	{"articles", "Articles", "Writing", "\uf1ea", "A", 0xf3b75c},
	{"newsletters", "Newsletters", "Writing", "\U000f0fb1", "N", 0xe3a3ff},
	{"directories", "Directories", "Writing", "\uf0ca", "D", 0x9ad0c2},

	{"issues", "Issues", "GitHub", "\uf41b", "i", 0x5fd8a3},
	{"prs", "Pull requests", "GitHub", "\uf407", "p", 0xa89cff},
	{"discussions", "Discussions", "GitHub", "\uf442", "d", 0x74b4ff},
	{"repos", "Repositories", "GitHub", "\uf401", "r", 0xc9c4dc},
	{"dotfiles", "Dotfiles", "GitHub", "\ue615", ".", 0x9fb4c8},
	{"lists", "Lists & digests", "GitHub", "\uf005", "*", 0xf3d35c},
	{"code", "Code", "GitHub", "\uf44f", "c", 0x8f8aa8},
	{"dependents", "Dependents", "GitHub", "\uf4f9", "u", 0x9ad0c2},

	{"registries", "Registries", "Packages", "\uf487", "P", 0x5fd8a3},
}

var groups []Group

var groupIndex map[string]int

// setGroups builds the sidebar: "all", then the built-in entries and the
// feed's own, keeping only those with items. A group no entry describes
// gets a plain one under "Other".
func setGroups(extra []feed.Group, used map[string]bool) {
	all := append([]Group(nil), builtinGroups...)
	at := map[string]int{}
	for i, g := range all {
		at[g.Key] = i
	}
	for _, fg := range extra {
		g := Group{Key: fg.ID, Name: fg.Name, Section: fg.Section, Glyph: fg.Icon, ASCII: fg.ASCII, Color: cSub}
		if i, ok := at[fg.ID]; ok {
			old := all[i]
			if g.Section == "" {
				g.Section = old.Section
			}
			if g.Glyph == "" {
				g.Glyph = old.Glyph
			}
			if g.ASCII == "" {
				g.ASCII = old.ASCII
			}
			g.Color = old.Color
		}
		if c, ok := parseHex(fg.Color); ok {
			g.Color = c
		}
		if g.Section == "" {
			g.Section = "Other"
		}
		if g.Glyph == "" {
			g.Glyph = "\uf111"
		}
		if g.ASCII == "" {
			g.ASCII = strings.ToLower(g.Name[:1])
		}
		if i, ok := at[fg.ID]; ok {
			all[i] = g
		} else {
			at[fg.ID] = len(all)
			all = append(all, g)
		}
	}
	var unknown []string
	for k := range used {
		if _, ok := at[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		name := strings.ToUpper(k[:1]) + strings.ReplaceAll(k[1:], "-", " ")
		all = append(all, Group{k, name, "Other", "\uf111", strings.ToLower(k[:1]), cSub})
	}
	// Keep the order of sections as they first appear, and the entries in
	// each section in their order.
	var secs []string
	bySec := map[string][]Group{}
	for _, g := range all {
		if !used[g.Key] {
			continue
		}
		if _, ok := bySec[g.Section]; !ok {
			secs = append(secs, g.Section)
		}
		bySec[g.Section] = append(bySec[g.Section], g)
	}
	groups = []Group{{"all", "Everything", "", "\uf0ac", "*", cAccentHi}}
	for _, s := range secs {
		groups = append(groups, bySec[s]...)
	}
	groupIndex = map[string]int{}
	for i, g := range groups {
		groupIndex[g.Key] = i
	}
}

func groupByKey(k string) *Group {
	if i, ok := groupIndex[k]; ok {
		return &groups[i]
	}
	return nil
}

// Glyphs used by the chrome. Each has an ASCII stand-in.
type glyphSet struct {
	dot, star, ban, check, search, sort, link, user, cal, eye, bolt, chart, sep, bar, up string
}

var nerdGlyphs = glyphSet{
	dot: "●", star: "", ban: "", check: "", search: "", sort: "",
	link: "", user: "\U000f0004", cal: "", eye: "", bolt: "", chart: "",
	sep: "·", bar: "▌", up: "↑",
}

var asciiGlyphs = glyphSet{
	dot: "*", star: "*", ban: "x", check: "v", search: "/", sort: "s",
	link: ">", user: "@", cal: "#", eye: "o", bolt: "!", chart: "#",
	sep: "-", bar: "|", up: "^",
}

func verdictColor(v string) RGB {
	switch v {
	case "real":
		return cGreen
	case "likely":
		return cAmber
	case "unverified":
		return cMuted
	}
	return cRed
}

func stateColor(s string) RGB {
	switch s {
	case "featured":
		return cAmber
	case "seen":
		return cBlue
	case "ignore":
		return cRed
	}
	return cAccentHi
}
