package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// Attr bits.
const (
	aBold uint8 = 1 << iota
	aItalic
	aUnder
	aFaint
)

// Cell is one terminal cell. W is 1 or 2, or 0 for the right half of a wide
// rune.
type Cell struct {
	R      string
	W      int8
	FG, BG RGB
	A      uint8
}

// Canvas is a grid the view draws into. Drawing into cells, rather than
// joining styled strings, keeps every column exact at every size, and gives
// the mouse handler the same coordinates the eye sees.
type Canvas struct {
	W, H int
	C    []Cell
}

// St is a text style. BG noColor keeps the background under the text.
type St struct {
	FG, BG RGB
	A      uint8
}

func fg(c RGB) St       { return St{FG: c, BG: noColor} }
func (s St) bold() St   { s.A |= aBold; return s }
func (s St) italic() St { s.A |= aItalic; return s }
func (s St) under() St  { s.A |= aUnder; return s }

// withBG fills in a background only where the style has none.
func (s St) withBG(bg RGB) St {
	if s.BG == noColor {
		s.BG = bg
	}
	return s
}
func (s St) on(bg RGB) St { s.BG = bg; return s }

func newCanvas(w, h int, bg RGB) *Canvas {
	c := &Canvas{W: w, H: h, C: make([]Cell, w*h)}
	for i := range c.C {
		c.C[i] = Cell{R: " ", W: 1, FG: cText, BG: bg}
	}
	return c
}

func (c *Canvas) at(x, y int) *Cell {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return nil
	}
	return &c.C[y*c.W+x]
}

// Fill clears a rectangle to a background.
func (c *Canvas) Fill(x, y, w, h int, bg RGB) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if p := c.at(i, j); p != nil {
				*p = Cell{R: " ", W: 1, FG: cText, BG: bg}
			}
		}
	}
}

// Tint changes only the background of a rectangle.
func (c *Canvas) Tint(x, y, w, h int, bg RGB) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if p := c.at(i, j); p != nil {
				p.BG = bg
			}
		}
	}
}

func runeW(r rune) int {
	if r < 0x20 {
		return 0
	}
	if r >= 0xe000 && r <= 0xf8ff || r >= 0xf0000 {
		return 1 // Nerd Font glyphs are one cell in a Mono font.
	}
	return runewidth.RuneWidth(r)
}

// strW is the width of s in cells.
func strW(s string) int {
	n := 0
	for _, r := range s {
		n += runeW(r)
	}
	return n
}

// Text draws s at x,y and never past x+maxw. It returns the columns used.
func (c *Canvas) Text(x, y int, s string, st St, maxw int) int {
	used := 0
	for _, r := range s {
		w := runeW(r)
		if w == 0 {
			continue
		}
		if used+w > maxw {
			if w == 2 && used < maxw {
				if p := c.at(x+used, y); p != nil {
					p.R, p.W = " ", 1
				}
				used++
			}
			break
		}
		p := c.at(x+used, y)
		if p == nil {
			used += w
			continue
		}
		// Writing over half of a wide rune clears the other half.
		if p.W == 0 {
			if q := c.at(x+used-1, y); q != nil {
				q.R, q.W = " ", 1
			}
		}
		p.R, p.W, p.FG, p.A = string(r), int8(w), st.FG, st.A
		if st.BG != noColor {
			p.BG = st.BG
		}
		if w == 2 {
			if q := c.at(x+used+1, y); q != nil {
				q.R, q.W, q.FG = "", 0, st.FG
				if st.BG != noColor {
					q.BG = st.BG
				}
			} else {
				p.R, p.W = " ", 1
			}
		}
		used += w
	}
	return used
}

// TextR draws s right-aligned so it ends at column right (exclusive).
func (c *Canvas) TextR(right, y int, s string, st St) int {
	w := strW(s)
	return c.Text(right-w, y, s, st, w)
}

// truncate shortens s to w cells with an ellipsis.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if strW(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := runeW(r)
		if used+rw > w-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return strings.TrimRight(b.String(), " ") + "…"
}

// wrap breaks s into lines of at most w cells, on spaces where it can.
func wrap(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line, lw := "", 0
		for _, word := range words {
			ww := strW(word)
			for ww > w { // a word longer than the line: hard break
				if lw > 0 {
					out = append(out, line)
					line, lw = "", 0
				}
				head, rest := splitAt(word, w)
				out = append(out, head)
				word, ww = rest, strW(rest)
			}
			switch {
			case lw == 0:
				line, lw = word, ww
			case lw+1+ww <= w:
				line += " " + word
				lw += 1 + ww
			default:
				out = append(out, line)
				line, lw = word, ww
			}
		}
		if lw > 0 {
			out = append(out, line)
		}
	}
	return out
}

func splitAt(s string, w int) (string, string) {
	used := 0
	for i, r := range s {
		rw := runeW(r)
		if used+rw > w {
			return s[:i], s[i:]
		}
		used += rw
	}
	return s, ""
}

type styleKey struct {
	fg, bg RGB
	a      uint8
}

var styleCache = map[styleKey]lipgloss.Style{}

func styleFor(k styleKey) lipgloss.Style {
	if s, ok := styleCache[k]; ok {
		return s
	}
	s := lipgloss.NewStyle().
		Foreground(lipgloss.Color(hex(k.fg))).
		Background(lipgloss.Color(hex(k.bg))).
		Bold(k.a&aBold != 0).Italic(k.a&aItalic != 0).
		Underline(k.a&aUnder != 0).Faint(k.a&aFaint != 0)
	styleCache[k] = s
	return s
}

func hex(c RGB) string { return fmt.Sprintf("#%06x", uint32(c)&0xffffff) }

// String renders the canvas as runs of lipgloss-styled text, one line per row.
func (c *Canvas) String() string {
	var b strings.Builder
	b.Grow(c.W * c.H * 4)
	var run strings.Builder
	for y := 0; y < c.H; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		var cur styleKey
		have := false
		flush := func() {
			if run.Len() > 0 {
				b.WriteString(styleFor(cur).Render(run.String()))
				run.Reset()
			}
		}
		for x := 0; x < c.W; x++ {
			p := &c.C[y*c.W+x]
			if p.W == 0 {
				continue
			}
			k := styleKey{p.FG, p.BG, p.A}
			if p.R == " " && p.A&aUnder == 0 {
				k.fg, k.a = cText, 0 // a blank cell only shows its background
				if have && cur.bg == k.bg && cur.a&aUnder == 0 {
					k = cur
				}
			}
			if !have || k != cur {
				flush()
				cur, have = k, true
			}
			run.WriteString(p.R)
		}
		flush()
	}
	return b.String()
}
