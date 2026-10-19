package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type rect struct{ x, y, w, h int }

func (r rect) has(x, y int) bool {
	return r.w > 0 && x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type sideRow struct{ y, idx int }
type hit struct{ y, x0, x1, idx, kind int }

type layout struct {
	compact      bool
	headerH      int
	side         rect
	list         rect
	preview      rect
	previewBelow bool
	rowsTop      int
	rowH         int
	listRows     int
	sideRows     []sideRow
	tabs         []hit
	triBtns      []hit
}

func (m *model) computeLayout() {
	w, h := m.w, m.h
	l := layout{}
	l.compact = h < 40 || w < 120
	l.headerH = 7
	if l.compact {
		l.headerH = 3
	}
	bodyY := l.headerH
	bodyH := max(0, h-l.headerH-1)
	sideW := 0
	switch {
	case w >= 150:
		sideW = 28
	case w >= 96:
		sideW = 24
	}
	if sideW > 0 {
		l.side = rect{0, bodyY, sideW, bodyH}
	}
	lx := sideW + 2
	lw := w - lx - 1
	switch {
	case w >= 150:
		pw := max(46, min(72, w*34/100))
		l.preview = rect{w - pw - 1, bodyY, pw, bodyH}
		lw = l.preview.x - lx - 2
		l.list = rect{lx, bodyY, lw, bodyH}
	case bodyH >= 22:
		ph := max(9, min(15, bodyH*42/100))
		l.list = rect{lx, bodyY, lw, bodyH - ph - 1}
		l.preview = rect{lx - 1, bodyY + bodyH - ph, lw + 1, ph}
		l.previewBelow = true
	default:
		l.list = rect{lx, bodyY, lw, bodyH}
	}
	l.rowH = 1
	if !l.compact && l.list.w >= 64 {
		l.rowH = 2
	}
	l.rowsTop = l.list.y + 3
	if l.compact {
		l.rowsTop = l.list.y + 2
	}
	l.listRows = max(1, (l.list.y+l.list.h-l.rowsTop)/l.rowH)
	if m.focus == focusSide && l.side.w == 0 || m.focus == focusPreview && l.preview.w == 0 {
		m.focus = focusList
	}
	m.lay = l
}

func easeOut(t float64) float64 {
	t = max(0, min(1, t))
	return 1 - math.Pow(1-t, 3)
}

func (m *model) introP(delay float64) float64 {
	e := time.Since(m.start).Seconds() / introDur.Seconds()
	return easeOut((e - delay) / (1 - delay*0.6))
}

func (m *model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = m.ds.Project.Name
	if m.ds.Project.Subtitle != "" {
		v.WindowTitle += " / " + m.ds.Project.Subtitle
	}
	if m.w <= 0 || m.h <= 0 {
		v.Content = ""
		return v
	}
	if m.lay.headerH == 0 {
		m.computeLayout()
	}
	m.now = time.Now()
	c := newCanvas(m.w, m.h, cInk)
	m.lay.sideRows, m.lay.tabs, m.lay.triBtns = nil, nil, nil
	if m.lay.compact {
		m.drawHeaderCompact(c)
	} else {
		m.drawHeader(c)
	}
	if m.lay.side.w > 0 {
		m.drawSide(c)
	}
	m.drawList(c)
	if m.lay.preview.w > 0 {
		m.drawPreview(c)
	}
	m.drawFooter(c)
	if m.help {
		m.drawHelp(c)
	}
	v.Content = c.String()
	return v
}

// ---------------------------------------------------------------- header

// wordColor is the gradient across the wordmark, t from 0 to 1, with the
// intro sweeping the colour in from the left.
func wordColor(t, p float64) RGB {
	color := lerp(lerp(cAccent, cAccentHi, t*1.2), cPink, max(0, t-0.35)*1.4)
	if p < 1 {
		color = lerp(cInk, color, easeOut(p*1.6-t*0.6))
	}
	return color
}

// drawWordmark draws the project name three rows high from the bitmap font
// in wordmark.go. It returns the columns used.
func drawWordmark(c *Canvas, x, y int, word string, p float64) int {
	glyphs := wordGlyphs(word)
	total := 0
	for i, bits := range glyphs {
		total += len(bits[0])
		if i > 0 {
			total++
		}
	}
	col := 0
	for i, bits := range glyphs {
		bw := len(bits[0])
		for cx := range bw {
			color := wordColor(float64(col+cx)/float64(max(1, total-1)), p)
			for row := range 3 {
				top := bits[row*2][cx] == '#'
				bot := bits[row*2+1][cx] == '#'
				ch := " "
				switch {
				case top && bot:
					ch = "█"
				case top:
					ch = "▀"
				case bot:
					ch = "▄"
				}
				if ch != " " {
					c.Text(x+col+cx, y+row, ch, fg(color), 1)
				}
			}
		}
		col += bw
		if i < len(glyphs)-1 {
			col++
		}
	}
	return total
}

func (m *model) drawHeader(c *Canvas) {
	p := m.introP(0)
	x := 3
	name := m.ds.Project.Name
	ww := 0
	if wordmarkOK(name) && wordmarkWidth(name) <= max(20, m.w/3) {
		ww = drawWordmark(c, x, 1, name, p)
	} else {
		// A long name is plain bold text.
		ww = m.drawNamePlain(c, x, 2, truncate(name, m.w/3), p)
	}
	tx := x + ww + 3
	c.Text(tx, 1, "MENTIONS TRACKER", fg(cFaint).bold(), 30)
	c.Text(tx, 2, m.sweepLine(), fg(cMuted), 60)
	if sub := m.ds.Project.Subtitle; sub != "" {
		n := c.Text(tx, 3, "/ ", fg(cFaint), 4)
		c.Text(tx+n, 3, sub, fg(cSub).italic(), 40)
	}

	textEnd := tx + max(strW(m.sweepLine()), 16) + 4
	avail := m.w - 3 - textEnd
	if len(m.hist.weeks) > 0 && avail >= 24 {
		m.drawHistogram(c, textEnd, 1, avail, 3, true)
	}
	m.drawPills(c, 3, 5, m.w-6)
}

func (m *model) sweepLine() string {
	var parts []string
	if !m.ds.Updated.IsZero() {
		parts = append(parts, "updated "+m.ds.Updated.Format("2006-01-02"))
	}
	n := m.ds.Sources
	if n == 1 {
		parts = append(parts, "1 source")
	} else {
		parts = append(parts, commaInt(n)+" sources")
	}
	if m.opt.Note != "" {
		parts = append(parts, m.opt.Note)
	}
	return strings.Join(parts, " · ")
}

// drawNamePlain writes the project name in bold, with the wordmark colours.
func (m *model) drawNamePlain(c *Canvas, x, y int, name string, p float64) int {
	r := []rune(name)
	used := 0
	for i, ch := range r {
		used += c.Text(x+used, y, string(ch), fg(wordColor(float64(i)/float64(max(1, len(r)-1)), p)).bold(), 4)
	}
	return used
}

func (m *model) drawHeaderCompact(c *Canvas) {
	p := m.introP(0)
	x := 2
	n := x + m.drawNamePlain(c, x, 0, truncate(m.ds.Project.Name, max(8, m.w/4)), p)
	if sub := m.ds.Project.Subtitle; sub != "" {
		n += c.Text(n, 0, " / ", fg(cFaint), 3)
		n += c.Text(n, 0, truncate(sub, 20), fg(cSub).italic(), 20)
	}
	avail := m.w - n - 4
	if len(m.hist.weeks) > 0 && avail >= 16 {
		hw := min(avail, max(len(m.hist.weeks), 24), 64)
		m.drawHistogram(c, m.w-2-hw, 0, hw, 1, false)
	}
	m.drawPills(c, 2, 1, m.w-4)
}

// drawHistogram draws mentions per week, w columns by rows rows. With
// labels it also writes an axis line under the bars.
func (m *model) drawHistogram(c *Canvas, x, y, w, rows int, labels bool) {
	weeks := m.hist.weeks
	n := len(weeks)
	// Pick a bar pitch: one bar per week with gaps, without gaps, or summed
	// buckets when the weeks outnumber the columns.
	labelW := 0
	if labels {
		labelW = 0
	}
	w -= labelW
	gap := 0
	var vals []int
	switch {
	case 2*n-1 <= w:
		gap = 1
		vals = weeks
	case n <= w:
		vals = weeks
	default:
		vals = make([]int, w)
		for i, v := range weeks {
			vals[i*w/n] += v
		}
	}
	peak := 1
	for _, v := range vals {
		peak = max(peak, v)
	}
	pitch := 1 + gap
	used := len(vals)*pitch - gap
	x0 := x + w - used // right-align, so "now" sits at the right edge
	levels := rows * 8
	blocks := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	for i, v := range vals {
		bp := m.introP(0.45 * float64(i) / float64(max(1, len(vals))))
		h := 0
		if v > 0 {
			// sqrt keeps the quiet weeks visible next to the launch spike.
			h = max(1, int(math.Round(math.Sqrt(float64(v)/float64(peak))*float64(levels)*bp)))
		}
		t := float64(h) / float64(levels)
		color := lerp(lerp(cAccentLo, cAccent, t*1.6), cPink, max(0, t-0.6)*2)
		if i == len(vals)-1 {
			color = cGreen
		}
		for r := range rows {
			cellLv := h - (rows-1-r)*8
			ch := ""
			switch {
			case cellLv >= 8:
				ch = blocks[8]
			case cellLv > 0:
				ch = blocks[cellLv]
			}
			if ch == "" {
				if r == rows-1 && rows > 1 {
					c.Text(x0+i*pitch, y+r, "▁", fg(cLineSoft), 1)
				}
				continue
			}
			c.Text(x0+i*pitch, y+r, ch, fg(color), 1)
		}
	}
	if !labels {
		return
	}
	ly := y + rows
	first := m.hist.first.Format("Jan 2006")
	c.Text(x0, ly, first, fg(cFaint), 12)
	mid := fmt.Sprintf("peak %d / week  ·  %d this week", m.hist.peak, weeks[n-1])
	c.Text(x0+(used-strW(mid))/2, ly, mid, fg(cMuted), used)
	c.TextR(x0+used, ly, "now", fg(cGreen))
}

// sectionPill is the pill label of a sidebar section: "Social & video"
// becomes "social", "GitHub" stays "GitHub".
func sectionPill(sec string) string {
	w := strings.Fields(sec)
	if len(w) == 0 {
		return "other"
	}
	first := w[0]
	if strings.ToUpper(first[:1]) == first[:1] && strings.ToLower(first[1:]) == first[1:] {
		return strings.ToLower(first)
	}
	return first
}

type pill struct {
	n     int
	label string
	color RGB
	glyph string
}

func (m *model) drawPills(c *Canvas, x, y, maxw int) {
	s := m.stats
	p := m.introP(0.05)
	pills := []pill{{s.mentions, "mentions", cAccentHi, ""}}
	for _, sec := range s.sections {
		label := sectionPill(sec.name)
		if sec.n == 1 && strings.HasSuffix(label, "s") && strings.ToLower(label) == label {
			label = strings.TrimSuffix(label, "s")
		}
		pills = append(pills, pill{sec.n, label, sec.color, sec.glyph})
	}
	pills = append(pills, pill{s.featured, "featured", cAmber, "\uf005"})
	ascii := m.g.dot == "*"
	cx := x
	end := x + maxw
	week := ""
	if s.week > 0 {
		week = fmt.Sprintf("%s %d this week", m.g.up, int(math.Round(float64(s.week)*p)))
	}
	for i, pl := range pills {
		num := commaInt(int(math.Round(float64(pl.n) * p)))
		body := " " + num + " " + pl.label + " "
		gw := 0
		if pl.glyph != "" && !ascii {
			gw = 2
		}
		pw := strW(body) + gw + 2
		reserve := 0
		if i < len(pills)-1 {
			reserve = 0
		}
		if cx+pw+reserve > end {
			break
		}
		bg := lerp(pl.color, cInk, 0.86)
		capL, capR := "", ""
		if ascii {
			capL, capR = " ", " "
		}
		c.Text(cx, y, capL, fg(bg).on(cInk), 1)
		xx := cx + 1
		c.Fill(xx, y, pw-2, 1, bg)
		if gw > 0 {
			c.Text(xx+1, y, pl.glyph, fg(pl.color).on(bg), 1)
			xx += 2
		}
		nn := c.Text(xx, y, " "+num, fg(pl.color).on(bg).bold(), 20)
		c.Text(xx+nn, y, " "+pl.label+" ", fg(cSub).on(bg), 20)
		c.Text(cx+pw-1, y, capR, fg(bg).on(cInk), 1)
		cx += pw + 1
	}
	if week != "" && cx+strW(week)+2 <= end {
		c.TextR(end, y, week, fg(cGreen))
	}
}

// ---------------------------------------------------------------- sidebar

func (m *model) drawSide(c *Canvas) {
	r := m.lay.side
	c.Fill(r.x, r.y, r.w, r.h, cPanel)
	focused := m.focus == focusSide
	labelC := cFaint
	if focused {
		labelC = cAccentHi
	}
	c.Text(r.x+2, r.y+1, "SOURCES", fg(labelC).bold(), r.w-4)
	hidden := "h shows noise"
	if m.showHidden {
		hidden = "noise shown"
	}
	if r.w >= 28 {
		c.TextR(r.x+r.w-2, r.y+1, hidden, fg(cFaint))
	}

	type entry struct {
		kind int // 0 group, 1 section, 2 blank
		idx  int
		text string
	}
	build := func(blanks, zeros bool) []entry {
		var out []entry
		sec := ""
		for i, g := range groups {
			if !zeros && i != m.groupIdx && i != 0 && m.groupCnt[g.Key] == 0 {
				continue
			}
			if g.Section != sec {
				sec = g.Section
				if blanks {
					out = append(out, entry{kind: 2})
				}
				out = append(out, entry{kind: 1, text: strings.ToUpper(sec)})
			}
			out = append(out, entry{kind: 0, idx: i})
		}
		return out
	}
	avail := r.h - 3
	es := build(true, true)
	if len(es) > avail {
		es = build(false, true)
	}
	if len(es) > avail {
		es = build(false, false)
	}
	// Scroll so the selected group stays in view.
	selPos := 0
	for i, e := range es {
		if e.kind == 0 && e.idx == m.groupIdx {
			selPos = i
		}
	}
	if selPos < m.sideOff {
		m.sideOff = selPos
	}
	if selPos >= m.sideOff+avail {
		m.sideOff = selPos - avail + 1
	}
	m.sideOff = max(0, min(m.sideOff, max(0, len(es)-avail)))

	y := r.y + 3
	for _, e := range es[m.sideOff:] {
		if y >= r.y+r.h {
			break
		}
		switch e.kind {
		case 1:
			c.Text(r.x+2, y, e.text, fg(cFaint), r.w-4)
		case 0:
			g := groups[e.idx]
			cnt := m.groupCnt[g.Key]
			sel := e.idx == m.groupIdx
			nameC, cntC, glyphC := cSub, cMuted, g.Color
			if cnt == 0 && e.idx != 0 {
				nameC, cntC, glyphC = cFaint, cFaint, lerp(g.Color, cPanel, 0.65)
			}
			if sel {
				bgc := cSel
				if !focused {
					bgc = lerp(cSel, cPanel, 0.45)
				}
				for i := 0; i < r.w-1; i++ {
					c.Tint(r.x+1+i, y, 1, 1, lerp(bgc, cPanel, float64(i)/float64(r.w)*0.7))
				}
				c.Text(r.x, y, "▌", fg(cAccent), 1)
				nameC, cntC = cText, cAccentHi
			}
			glyph := g.Glyph
			if m.g.dot == "*" {
				glyph = g.ASCII
			}
			c.Text(r.x+2, y, glyph, fg(glyphC), 1)
			cs := commaInt(cnt)
			name := truncate(g.Name, r.w-8-strW(cs))
			st := fg(nameC)
			if sel {
				st = st.bold()
			}
			c.Text(r.x+4, y, name, st, r.w-6)
			c.TextR(r.x+r.w-2, y, cs, fg(cntC))
			m.lay.sideRows = append(m.lay.sideRows, sideRow{y, e.idx})
		}
		y++
	}
}

// ---------------------------------------------------------------- list

func (m *model) drawList(c *Canvas) {
	l := m.lay
	r := l.list
	if r.w < 10 || r.h < 3 {
		return
	}
	// View tabs.
	x := r.x
	for i, name := range viewNames {
		label := name + " " + commaInt(m.viewCnt[i])
		w := strW(label) + 2
		if x+w > r.x+r.w-16 && i > 0 {
			break
		}
		if i == m.view {
			bg := cAccentLo
			c.Text(x, r.y, " "+label+" ", fg(cText).on(bg).bold(), w)
		} else {
			c.Text(x+1, r.y, name, fg(cMuted), w)
			c.Text(x+1+strW(name)+1, r.y, commaInt(m.viewCnt[i]), fg(cFaint), w)
		}
		m.lay.tabs = append(m.lay.tabs, hit{r.y, x, x + w, i, 0})
		x += w + 1
	}
	sortLabel := m.g.sort + " " + sortNames[m.sortMode]
	if g := groups[m.groupIdx]; g.Key != "all" && r.w >= 70 {
		glyph := g.Glyph
		if m.g.dot == "*" {
			glyph = g.ASCII
		}
		gl := glyph + " " + g.Name + "   "
		sx := r.x + r.w - strW(sortLabel) - strW(gl)
		if sx > x+1 {
			c.Text(sx, r.y, glyph, fg(g.Color), 1)
			c.Text(sx+2, r.y, g.Name, fg(cSub), 30)
		}
	}
	sx := r.x + r.w - strW(sortLabel)
	if sx > x {
		c.Text(sx, r.y, sortLabel, fg(cMuted), 30)
		m.lay.tabs = append(m.lay.tabs, hit{r.y, sx, sx + strW(sortLabel), 0, 1})
	}

	// Search line, or a quiet rule.
	sy := r.y + 1
	if m.searching || m.query != "" {
		bg := cLineSoft
		if m.searching {
			bg = lerp(cAccentLo, cInk, 0.35)
		}
		c.Fill(r.x, sy, r.w, 1, bg)
		n := c.Text(r.x+1, sy, m.g.search+" ", fg(cAccentHi).on(bg), 3)
		qn := c.Text(r.x+1+n, sy, m.query, fg(cText).on(bg).bold(), r.w-20)
		if m.searching {
			c.Text(r.x+1+n+qn, sy, "▏", fg(cAccentHi).on(bg), 1)
		}
		info := commaInt(len(m.items)) + " found"
		if len(m.items) == 1 {
			info = "1 found"
		}
		c.TextR(r.x+r.w-1, sy, info, fg(cMuted).on(bg))
	} else if !l.compact {
		for i := 0; i < r.w; i++ {
			c.Text(r.x+i, sy+1, "─", fg(lerp(cLineSoft, cInk, float64(i)/float64(r.w)*0.9)), 1)
		}
	}

	rowsW := r.w - 2 // the last column holds the scrollbar
	if len(m.items) == 0 {
		msg := "No mentions here"
		hint := "Press esc to clear the search, or v to change the view."
		if m.query == "" {
			hint = "Press v to change the view, or h to show hidden verdicts."
		}
		cy := l.rowsTop + max(0, (l.listRows*l.rowH)/2-2)
		c.Text(r.x+(rowsW-strW(msg))/2, cy, msg, fg(cSub).bold(), rowsW)
		hint = truncate(hint, rowsW)
		c.Text(r.x+(rowsW-strW(hint))/2, cy+1, hint, fg(cFaint), rowsW)
		return
	}
	for i := 0; i < l.listRows; i++ {
		idx := m.off + i
		if idx >= len(m.items) {
			break
		}
		y := l.rowsTop + i*l.rowH
		m.drawRow(c, m.items[idx], idx == m.sel, r.x, y, rowsW, l.rowH)
	}
	// Scrollbar.
	trackH := l.listRows * l.rowH
	if len(m.items) > l.listRows && trackH > 2 {
		sbx := r.x + r.w - 1
		th := max(1, trackH*l.listRows/len(m.items))
		ty := (trackH - th) * m.off / max(1, len(m.items)-l.listRows)
		for i := range trackH {
			ch, col := "│", cLineSoft
			if i >= ty && i < ty+th {
				ch, col = "┃", cAccentLo
				if m.focus == focusList {
					col = cAccent
				}
			}
			c.Text(sbx, l.rowsTop+i, ch, fg(col), 1)
		}
	}
}

func typeLabel(it *Mention) string {
	switch {
	case it.Source == "github" && it.Type == "code":
		if len(it.Files) == 1 {
			return "1 file"
		}
		return fmt.Sprintf("%d files", len(it.Files))
	case it.Type == "pr":
		return "pull request"
	}
	return it.Type
}

func (m *model) drawRow(c *Canvas, it *Mention, sel bool, x, y, w, rh int) {
	st := m.tri.Of(it)
	g := groupByKey(it.Group)
	gc := cMuted
	glyph := "?"
	if g != nil {
		gc, glyph = g.Color, g.Glyph
		if m.g.dot == "*" {
			glyph = g.ASCII
		}
	}
	// Background: featured rows glow amber, the selection fades in.
	a := 1.0
	if sel && !m.selAt.IsZero() {
		a = easeOut(float64(time.Since(m.selAt)) / float64(fadeDur))
	}
	for i := range w {
		t := float64(i) / float64(max(1, w-1))
		base := cInk
		if st == "featured" {
			base = lerp(cFeatBG, cInk, t*0.85)
		}
		bg := base
		if sel {
			target := lerp(cSel, cInk, math.Pow(t, 1.4)*0.8)
			if m.focus != focusList {
				target = lerp(target, base, 0.45)
			}
			bg = lerp(base, target, a)
		}
		c.Tint(x+i, y, 1, rh, bg)
	}
	if sel {
		bar := lerp(cInk, cAccent, a)
		for j := range rh {
			c.Text(x, y+j, "▌", fg(bar), 1)
		}
	} else if st == "featured" {
		for j := range rh {
			c.Text(x, y+j, "▌", fg(cAmber), 1)
		}
	}

	// State mark.
	switch st {
	case "new":
		c.Text(x+2, y, m.g.dot, fg(cAccentHi), 1)
	case "featured":
		c.Text(x+2, y, m.g.star, fg(cAmber), 1)
	case "ignore":
		c.Text(x+2, y, m.g.ban, fg(lerp(cRed, cInk, 0.4)), 1)
	}
	c.Text(x+4, y, glyph, fg(gc), 1)

	titleC := cText
	switch st {
	case "seen":
		titleC = cSub
	case "ignore":
		titleC = cFaint
	case "featured":
		titleC = 0xf8dcae
	}
	tst := fg(titleC)
	if st == "new" || st == "featured" || sel {
		tst = tst.bold()
	}
	badgeC := []RGB{cMuted, cGreen, cAmber}[it.BadgeTier]
	if st == "ignore" {
		badgeC = cFaint
	}
	tx := x + 6
	if rh >= 2 {
		bw := strW(it.Badge)
		titleW := w - 6 - bw - 3
		m.drawTitle(c, it, tx, y, titleW, tst)
		if bw > 0 {
			c.TextR(x+w-1, y, it.Badge, fg(badgeC))
		}
		// Second line: author, date, type, verdict.
		meta := []struct {
			s string
			c RGB
		}{}
		if it.Author != "" {
			meta = append(meta, struct {
				s string
				c RGB
			}{truncate(it.Author, 28), cMuted})
		}
		meta = append(meta, struct {
			s string
			c RGB
		}{relTime(it.T, m.now), cFaint})
		meta = append(meta, struct {
			s string
			c RGB
		}{typeLabel(it), cFaint})
		if it.Verdict != "real" && it.Verdict != "" {
			meta = append(meta, struct {
				s string
				c RGB
			}{it.Verdict, lerp(verdictColor(it.Verdict), cInk, 0.25)})
		}
		mx := tx
		for i, p := range meta {
			if i > 0 {
				mx += c.Text(mx, y+1, "  ·  ", fg(cFaint), x+w-mx)
			}
			mx += c.Text(mx, y+1, p.s, fg(p.c), x+w-mx-1)
		}
		return
	}
	// One line: title, then author, date and badge in columns.
	badgeW := 12
	dateW := 5
	authW := 18
	if w < 90 {
		authW = 14
	}
	if w < 64 {
		authW = 0
	}
	if w < 48 {
		badgeW = 0
	}
	titleW := w - 6 - badgeW - dateW - authW - 5
	m.drawTitle(c, it, tx, y, titleW, tst)
	rx := x + w - 1
	if badgeW > 0 && it.Badge != "" {
		c.TextR(rx, y, truncate(it.Badge, badgeW), fg(badgeC))
	}
	rx -= badgeW + 1
	c.TextR(rx, y, relTime(it.T, m.now), fg(cFaint))
	rx -= dateW + 1
	if authW > 0 {
		c.Text(rx-authW, y, truncate(it.Author, authW-1), fg(cMuted), authW-1)
	}
}

// drawTitle writes a title and lifts the runes the search matched.
func (m *model) drawTitle(c *Canvas, it *Mention, x, y, w int, st St) {
	title := truncate(it.Title, w)
	pos := m.matches[it]
	if len(pos) == 0 {
		c.Text(x, y, title, st, w)
		return
	}
	set := map[int]bool{}
	for _, p := range pos {
		set[p] = true
	}
	hl := fg(cAccentHi).bold().under()
	used := 0
	for i, r := range []rune(title) {
		s := st
		if set[i] && r != '…' {
			s = hl
		}
		used += c.Text(x+used, y, string(r), s, w-used)
	}
}

func relTime(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := now.Sub(t)
	days := int(d.Hours() / 24)
	switch {
	case days < 1:
		return "today"
	case days < 7:
		return fmt.Sprintf("%dd", days)
	case days < 60:
		return fmt.Sprintf("%dw", days/7)
	case days < 365:
		return fmt.Sprintf("%dmo", days/30)
	}
	return fmt.Sprintf("%dy", days/365)
}

func relLong(t, now time.Time) string {
	if t.IsZero() {
		return "no date"
	}
	days := int(now.Sub(t).Hours() / 24)
	plural := func(n int, u string) string {
		if n == 1 {
			return "1 " + u + " ago"
		}
		return fmt.Sprintf("%d %ss ago", n, u)
	}
	switch {
	case days < 1:
		return "today"
	case days < 7:
		return plural(days, "day")
	case days < 60:
		return plural(days/7, "week")
	case days < 365:
		return plural(days/30, "month")
	}
	return plural(days/365, "year")
}

// ---------------------------------------------------------------- preview

type seg struct {
	s  string
	st St
	// btn marks a triage button: index+1.
	btn int
}

type pline []seg

var metricNames = map[string]string{
	"views": "views", "likes": "likes", "comments": "comments", "stars": "stars", "forks": "forks",
	"reactions": "reactions", "replies": "replies", "reposts": "reposts", "score": "score",
	"upvotes": "upvotes", "points": "points", "votes": "votes", "popularity": "popularity",
	"boosts": "boosts", "30d": "installs, 30 days", "90d": "installs, 90 days", "365d": "installs, 1 year",
	"bookmarks": "bookmarks", "files": "files",
}

var metricOrderFull = []string{"views", "30d", "90d", "365d", "stars", "forks", "likes", "reactions",
	"score", "points", "upvotes", "votes", "popularity", "reposts", "boosts", "bookmarks", "comments", "replies", "files"}

var globalMax = map[string]float64{}

func (m *model) metricMax(k string) float64 {
	if v, ok := globalMax[k]; ok {
		return v
	}
	mx := 1.0
	for _, it := range m.ds.Visible {
		mx = max(mx, it.Engagement[k])
	}
	globalMax[k] = mx
	return mx
}

func (m *model) previewLines(it *Mention, w int) []pline {
	var out []pline
	blank := func() { out = append(out, pline{}) }
	label := func(s string) { out = append(out, pline{{s: strings.ToUpper(s), st: fg(cFaint).bold()}}) }

	g := groupByKey(it.Group)
	gname, gc, glyph := it.Source, cMuted, "?"
	if g != nil {
		gname, gc, glyph = g.Name, g.Color, g.Glyph
		if m.g.dot == "*" {
			glyph = g.ASCII
		}
	}
	chipBG := lerp(gc, cCard, 0.82)
	vc := verdictColor(it.Verdict)
	vbg := lerp(vc, cCard, 0.84)
	head := pline{
		{s: " " + glyph + " " + gname + " ", st: fg(gc).on(chipBG).bold()},
		{s: "  " + typeLabel(it) + "  ", st: fg(cMuted)},
	}
	if it.Verdict != "" {
		head = append(head, seg{s: " " + it.Verdict + " ", st: fg(vc).on(vbg)})
	}
	out = append(out, head)
	blank()
	tl := wrap(it.Title, w)
	if len(tl) > 5 {
		tl = append(tl[:4], truncate(strings.Join(tl[4:], " "), w))
	}
	tc := cText
	if m.tri.Of(it) == "featured" {
		tc = 0xf8dcae
	}
	for _, l := range tl {
		out = append(out, pline{{s: l, st: fg(tc).bold()}})
	}
	blank()
	if it.Author != "" {
		out = append(out, pline{{s: m.g.user + "  ", st: fg(cFaint)}, {s: truncate(it.Author, w-3), st: fg(cSub)}})
	}
	date := "no date"
	if !it.T.IsZero() {
		date = it.T.Format("Jan 2, 2006")
	}
	out = append(out, pline{{s: m.g.cal + "  ", st: fg(cFaint)}, {s: date, st: fg(cSub)},
		{s: "  ·  " + relLong(it.T, m.now), st: fg(cMuted)}})

	// Snippet, or the file list of a code row.
	blank()
	if len(it.Files) > 0 {
		label(fmt.Sprintf("Files  %d", len(it.Files)))
		show := it.Files
		if len(show) > 12 {
			show = show[:12]
		}
		for _, f := range show {
			out = append(out, pline{{s: "  " + truncate(f, w-2), st: fg(cSub)}})
		}
		if more := len(it.Files) - len(show); more > 0 {
			out = append(out, pline{{s: fmt.Sprintf("  and %d more", more), st: fg(cMuted).italic()}})
		}
	} else if it.Snippet != "" && it.Snippet != it.Title {
		label("Snippet")
		for _, l := range wrap(cleanKeepLines(it.Snippet), w) {
			out = append(out, pline{{s: l, st: fg(cSub)}})
		}
	}
	if it.LinkTitle != "" {
		out = append(out, pline{{s: m.g.link + " ", st: fg(cFaint)}, {s: truncate(it.LinkTitle, w-2), st: fg(cMuted).italic()}})
	}

	// Engagement.
	blank()
	label("Engagement")
	any := false
	nameW := 0
	for _, k := range metricOrderFull {
		if _, ok := it.Engagement[k]; ok {
			nameW = max(nameW, strW(metricNames[k]))
		}
	}
	for _, k := range metricOrderFull {
		v, ok := it.Engagement[k]
		if !ok {
			continue
		}
		any = true
		val := commaInt(int(math.Round(v)))
		barW := max(4, w-nameW-2-10)
		frac := 0.0
		if v > 0 {
			frac = math.Log10(v+1) / math.Log10(m.metricMax(k)+1)
		}
		fill := int(math.Round(frac * float64(barW)))
		if v > 0 {
			fill = max(1, fill)
		}
		line := pline{{s: padRight(metricNames[k], nameW) + "  ", st: fg(cMuted)}}
		for i := range barW {
			if i < fill {
				t := float64(i) / float64(max(1, barW-1))
				line = append(line, seg{s: "━", st: fg(lerp(cAccent, cPink, t))})
			} else {
				line = append(line, seg{s: "─", st: fg(cLine)})
			}
		}
		line = append(line, seg{s: padLeft(val, 10), st: fg(cText).bold()})
		out = append(out, line)
	}
	if !any {
		out = append(out, pline{{s: "No counts for this source.", st: fg(cFaint).italic()}})
	}

	blank()
	label("Link")
	for _, l := range wrap(it.URL, w) {
		out = append(out, pline{{s: l, st: fg(cBlue).under()}})
	}

	blank()
	label("Triage")
	cur := m.tri.Of(it)
	btns := []struct{ key, name, glyph string }{
		{"new", "new", m.g.dot}, {"seen", "seen", m.g.check}, {"featured", "featured", m.g.star}, {"ignore", "ignore", m.g.ban},
	}
	var bl pline
	for i, b := range btns {
		s := " " + b.glyph + " " + b.name + " "
		var st St
		if b.key == cur {
			sc := stateColor(b.key)
			st = fg(sc).on(lerp(sc, cCard, 0.78)).bold()
		} else {
			st = fg(cMuted).on(cLineSoft)
		}
		if i > 0 {
			bl = append(bl, seg{s: " ", st: fg(cCard)})
		}
		bl = append(bl, seg{s: s, st: st, btn: i + 1})
	}
	out = append(out, bl)
	out = append(out, pline{{s: "n new · space seen · f feature · x ignore", st: fg(cFaint)}})

	if it.Reason != "" {
		blank()
		label("Why it counts")
		for _, l := range wrap(it.Reason, w) {
			out = append(out, pline{{s: l, st: fg(cMuted)}})
		}
	}
	return out
}

func cleanKeepLines(s string) string {
	var parts []string
	for _, p := range strings.Split(s, "\n") {
		parts = append(parts, cleanText(p))
	}
	return strings.Join(parts, "\n")
}

func padRight(s string, w int) string { return s + strings.Repeat(" ", max(0, w-strW(s))) }
func padLeft(s string, w int) string  { return strings.Repeat(" ", max(0, w-strW(s))) + s }

func (m *model) drawPreview(c *Canvas) {
	r := m.lay.preview
	focused := m.focus == focusPreview
	bgc := cCard
	if focused {
		bgc = lerp(cCard, cAccent, 0.07)
	}
	drawCard(c, r, bgc, cInk)

	it := m.current()
	padX := 3
	if m.lay.previewBelow {
		padX = 2
	}
	cw := r.w - padX*2
	if it == nil {
		msg := "Nothing selected"
		c.Text(r.x+(r.w-strW(msg))/2, r.y+r.h/2, msg, fg(cFaint), r.w-2)
		return
	}
	lines := m.previewLines(it, cw)
	top := r.y + 1
	if !m.lay.previewBelow {
		top = r.y + 2
	}
	visible := r.y + r.h - 2 - top
	maxOff := max(0, len(lines)-visible)
	m.pvOff = min(m.pvOff, maxOff)
	for i := 0; i < visible; i++ {
		li := m.pvOff + i
		if li >= len(lines) {
			break
		}
		x := r.x + padX
		for _, s := range lines[li] {
			n := c.Text(x, top+i, s.s, s.st.withBG(bgc), r.x+r.w-padX-x)
			if s.btn > 0 {
				m.lay.triBtns = append(m.lay.triBtns, hit{top + i, x, x + n, s.btn - 1, 0})
			}
			x += n
		}
	}
	if maxOff > 0 {
		more := "more ↓"
		if m.pvOff >= maxOff {
			more = "end"
		} else if m.pvOff > 0 {
			more = fmt.Sprintf("%d%% ↓", int(math.Round(float64(m.pvOff)/float64(maxOff)*100)))
		}
		c.TextR(r.x+r.w-3, r.y+r.h-2, strings.TrimSpace(more), fg(cFaint).on(bgc))
	}
}

// ---------------------------------------------------------------- footer

func (m *model) drawFooter(c *Canvas) {
	y := m.h - 1
	c.Fill(0, y, m.w, 1, cInkDeep)
	hints := [][2]string{
		{"j k", "move"}, {"/", "search"}, {"tab", "focus"}, {"s", "sort"}, {"v", "view"},
		{"f", "feature"}, {"x", "ignore"}, {"space", "seen"}, {"o", "open"}, {"y", "copy"},
		{"h", "hidden"}, {"?", "help"}, {"q", "quit"},
	}
	if m.opt.NoQuit {
		hints = hints[:len(hints)-1]
	}
	if m.searching {
		hints = [][2]string{{"type", "filter"}, {"enter", "keep"}, {"esc", "clear"}, {"ctrl+u", "erase"}}
	}
	right := ""
	rc := cMuted
	if time.Now().Before(m.toastUntil) && m.toast != "" {
		right, rc = m.toast, m.toastColor
	} else if len(m.items) > 0 {
		right = commaInt(m.sel+1) + " / " + commaInt(len(m.items))
	}
	rw := strW(right)
	x := 1
	for _, h := range hints {
		need := strW(h[0]) + 2 + strW(h[1]) + 3
		if x+need > m.w-rw-4 {
			break
		}
		x += c.Text(x, y, " "+h[0]+" ", fg(cAccentHi).on(cLineSoft).bold(), 20)
		x += c.Text(x, y, " "+h[1], fg(cMuted), 20)
		x += 2
	}
	if right != "" {
		c.TextR(m.w-2, y, right, fg(rc).bold())
	}
}

// ---------------------------------------------------------------- help

func (m *model) drawHelp(c *Canvas) {
	keys := [][2]string{
		{"j k  ↓ ↑", "move"},
		{"g G", "first, last"},
		{"ctrl+d ctrl+u", "page down, up"},
		{"tab", "switch the pane focus"},
		{"[ ]", "previous, next source"},
		{"/", "fuzzy search, enter keeps it"},
		{"esc", "clear the search"},
		{"s", "sort: newest, engagement, source"},
		{"v", "view: all, new, featured, ignored"},
		{"h", "show or hide noise, automation, own"},
		{"o  enter", "open the link"},
		{"y", "copy the link (OSC 52)"},
		{"f", "feature, again to unfeature"},
		{"x", "ignore"},
		{"space", "mark seen and go to the next"},
		{"n", "mark new"},
		{"q", "quit"},
	}
	if m.opt.NoQuit {
		keys = keys[:len(keys)-1]
	}
	w := min(62, m.w-4)
	h := min(len(keys)+6, m.h-2)
	x := (m.w - w) / 2
	y := (m.h - h) / 2
	// Dim what is behind the card.
	for i := range c.C {
		c.C[i].FG = lerp(c.C[i].FG, cInkDeep, 0.6)
		c.C[i].BG = lerp(c.C[i].BG, cInkDeep, 0.5)
	}
	drawCard(c, rect{x, y, w, h}, cCard, lerp(cInk, cInkDeep, 0.5))
	c.Text(x+3, y+1, "Keys", fg(cText).bold(), w-6)
	c.TextR(x+w-3, y+1, "any key closes", fg(cFaint))
	for i, k := range keys {
		ry := y + 3 + i
		if ry >= y+h-1 {
			break
		}
		c.Text(x+3, ry, k[0], fg(cAccentHi).bold(), 16)
		c.Text(x+20, ry, k[1], fg(cSub), w-23)
	}
	if m.tri.Memory {
		c.TextR(x+w-3, y+h-2, "triage stays in memory", fg(cAmber))
	}
}

// drawCard paints a solid card with half-block edges, so the card ends on a
// clean line half a cell outside its text, with no border stroke.
func drawCard(c *Canvas, r rect, bg, outside RGB) {
	c.Fill(r.x+1, r.y+1, r.w-2, r.h-2, bg)
	e := fg(bg).on(outside)
	for i := 1; i < r.w-1; i++ {
		c.Text(r.x+i, r.y, "▄", e, 1)
		c.Text(r.x+i, r.y+r.h-1, "▀", e, 1)
	}
	for j := 1; j < r.h-1; j++ {
		c.Text(r.x, r.y+j, "▐", e, 1)
		c.Text(r.x+r.w-1, r.y+j, "▌", e, 1)
	}
	c.Text(r.x, r.y, "▗", e, 1)
	c.Text(r.x+r.w-1, r.y, "▖", e, 1)
	c.Text(r.x, r.y+r.h-1, "▝", e, 1)
	c.Text(r.x+r.w-1, r.y+r.h-1, "▘", e, 1)
}
