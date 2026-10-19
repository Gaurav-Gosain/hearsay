package ui

import (
	"sort"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

const (
	focusSide = iota
	focusList
	focusPreview
)

var sortNames = []string{"newest", "engagement", "source"}
var viewNames = []string{"All", "New", "Featured", "Ignored"}

type tickMsg time.Time

type model struct {
	w, h int
	ds   *Dataset
	tri  *Triage
	g    glyphSet
	opt  Options

	showHidden bool
	groupIdx   int
	sortMode   int
	view       int
	query      string
	searching  bool

	items   []*Mention
	matches map[*Mention][]int
	sel     int
	off     int
	sideOff int
	pvOff   int
	focus   int
	help    bool

	resetSel bool

	toast      string
	toastColor RGB
	toastUntil time.Time

	start    time.Time
	selAt    time.Time
	now      time.Time
	ticking  bool
	viewCnt  [4]int
	groupCnt map[string]int

	stats stats
	hist  hist

	lay layout // last drawn geometry, for the mouse
}

type stats struct {
	mentions, featured, week int
	sections                 []secStat
}

// secStat is one pill: the mentions in one sidebar section.
type secStat struct {
	name  string
	n     int
	color RGB
	glyph string
}

// sectionLooks are the pill colours and glyphs of the built-in sections.
// Any other section takes its first group's.
var sectionLooks = map[string]struct {
	color RGB
	glyph string
}{
	"Social & video": {cBlue, "\uf075"},
	"Writing":        {cAmber, "\uf1ea"},
	"GitHub":         {0xd8d4ea, "\uf09b"},
	"Packages":       {cGreen, "\uf487"},
}

type hist struct {
	weeks []int
	first time.Time
	peak  int
}

// Options change how the program behaves on its host.
type Options struct {
	// ASCII uses plain marks in place of Nerd Font glyphs.
	ASCII bool
	// Note is a word for the header, such as "demo" or "read only".
	Note string
	// Open opens a link. Nil means the link cannot be opened.
	Open func(url string) tea.Cmd
	// Copy puts a link on the clipboard. Nil means OSC 52.
	Copy func(url string) tea.Cmd
	// NoQuit turns q off, for a host where quitting leaves a dead page.
	NoQuit bool
}

// New makes the program's model. NewDataset must run first.
func New(ds *Dataset, tri *Triage, opt Options) tea.Model {
	return newModel(ds, tri, opt)
}

// weight is the number of mentions a row stands for: a code search hit
// counts each matching file.
func weight(it *Mention) int { return max(1, len(it.Files)) }

func newModel(ds *Dataset, tri *Triage, opt Options) *model {
	m := &model{ds: ds, tri: tri, opt: opt, focus: focusList, sortMode: 0,
		start: time.Now(), now: time.Now(), matches: map[*Mention][]int{}}
	m.g = nerdGlyphs
	if opt.ASCII {
		m.g = asciiGlyphs
	}
	m.computeStats()
	m.rebuild()
	return m
}

func (m *model) pool() []*Mention {
	if !m.showHidden {
		return m.ds.Visible
	}
	out := make([]*Mention, 0, len(m.ds.Visible)+len(m.ds.Hidden))
	out = append(out, m.ds.Visible...)
	return append(out, m.ds.Hidden...)
}

func (m *model) computeStats() {
	var s stats
	weekAgo := m.now.AddDate(0, 0, -7)
	var first time.Time
	secAt := map[string]int{}
	for _, g := range groups[1:] {
		if _, ok := secAt[g.Section]; !ok {
			secAt[g.Section] = len(s.sections)
			st := secStat{name: g.Section, color: g.Color, glyph: g.Glyph}
			if l, ok := sectionLooks[g.Section]; ok {
				st.color, st.glyph = l.color, l.glyph
			}
			s.sections = append(s.sections, st)
		}
	}
	for _, it := range m.ds.Visible {
		w := weight(it)
		s.mentions += w
		if g := groupByKey(it.Group); g != nil {
			if i, ok := secAt[g.Section]; ok {
				s.sections[i].n += w
			}
		}
		if m.tri.Of(it) == "featured" {
			s.featured++
		}
		if !it.T.IsZero() {
			if it.T.After(weekAgo) {
				s.week += w
			}
			if first.IsZero() || it.T.Before(first) {
				first = it.T
			}
		}
	}
	m.stats = s
	if first.IsZero() {
		m.hist = hist{}
		return
	}
	first = weekStart(first)
	n := int(weekStart(m.now).Sub(first).Hours()/(24*7)) + 1
	h := hist{weeks: make([]int, n), first: first}
	for _, it := range m.ds.Visible {
		if it.T.IsZero() {
			continue
		}
		i := int(weekStart(it.T).Sub(first).Hours() / (24 * 7))
		if i >= 0 && i < n {
			h.weeks[i] += weight(it)
		}
	}
	for _, v := range h.weeks {
		h.peak = max(h.peak, v)
	}
	m.hist = h
}

func weekStart(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	wd := (int(t.Weekday()) + 6) % 7 // Monday first
	return t.AddDate(0, 0, -wd)
}

func (m *model) inView(it *Mention, v int) bool {
	st := m.tri.Of(it)
	switch v {
	case 1:
		return st == "new"
	case 2:
		return st == "featured"
	case 3:
		return st == "ignore"
	}
	return st != "ignore"
}

// rebuild filters and sorts the list. It keeps the selected row when it can.
func (m *model) rebuild() {
	var keep *Mention
	if m.resetSel {
		m.resetSel = false
		m.sel, m.off = 0, 0
		m.selAt = time.Now()
	} else if m.sel >= 0 && m.sel < len(m.items) {
		keep = m.items[m.sel]
	}
	terms := queryTerms(m.query)
	pool := m.pool()
	m.matches = map[*Mention][]int{}
	scores := map[*Mention]int{}
	m.groupCnt = map[string]int{}
	m.viewCnt = [4]int{}
	gkey := groups[m.groupIdx].Key
	items := m.items[:0:0]
	for _, it := range pool {
		score, pos, ok := matchItem(it, terms)
		if !ok {
			continue
		}
		for v := range 4 {
			if m.inView(it, v) && (gkey == "all" || it.Group == gkey) {
				m.viewCnt[v]++
			}
		}
		if !m.inView(it, m.view) {
			continue
		}
		m.groupCnt[it.Group]++
		m.groupCnt["all"]++
		if gkey != "all" && it.Group != gkey {
			continue
		}
		items = append(items, it)
		scores[it] = score
		if len(pos) > 0 {
			m.matches[it] = pos
		}
	}
	less := m.sortLess()
	if len(terms) > 0 {
		sort.SliceStable(items, func(i, j int) bool {
			a, b := scores[items[i]], scores[items[j]]
			if a != b {
				return a > b
			}
			return less(items[i], items[j])
		})
	} else {
		sort.SliceStable(items, func(i, j int) bool { return less(items[i], items[j]) })
	}
	m.items = items
	m.sel = 0
	if keep != nil {
		for i, it := range items {
			if it == keep {
				m.sel = i
				break
			}
		}
	}
	m.clamp()
}

func (m *model) sortLess() func(a, b *Mention) bool {
	newest := func(a, b *Mention) bool {
		if a.T.IsZero() != b.T.IsZero() {
			return !a.T.IsZero()
		}
		if !a.T.Equal(b.T) {
			return a.T.After(b.T)
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Title < b.Title
	}
	switch m.sortMode {
	case 1:
		return func(a, b *Mention) bool {
			if a.Score != b.Score {
				return a.Score > b.Score
			}
			return newest(a, b)
		}
	case 2:
		return func(a, b *Mention) bool {
			ga, gb := groupIndex[a.Group], groupIndex[b.Group]
			if ga != gb {
				return ga < gb
			}
			return newest(a, b)
		}
	}
	return newest
}

func queryTerms(q string) [][]rune {
	var out [][]rune
	for _, f := range strings.Fields(q) {
		r := []rune(strings.ToLower(f))
		for i := range r {
			r[i] = unicode.ToLower(r[i])
		}
		out = append(out, r)
	}
	return out
}

// matchItem needs every term to match: fuzzily in the title, author or
// source, or as a plain substring of the snippet and URL.
func matchItem(it *Mention, terms [][]rune) (int, []int, bool) {
	total := 0
	var pos []int
	for _, t := range terms {
		s, p, ok := fuzzy(it.hayLower, t)
		if ok && s > len(t) { // a match that is not scattered noise
			total += s
			for _, i := range p {
				if i < len(it.titleRunes) {
					pos = append(pos, i)
				}
			}
			continue
		}
		if strings.Contains(it.snipLower, string(t)) {
			total += 5
			continue
		}
		return 0, nil, false
	}
	sort.Ints(pos)
	return total, pos, true
}

func (m *model) clamp() {
	if m.sel >= len(m.items) {
		m.sel = len(m.items) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
	rows := m.lay.listRows
	if rows <= 0 {
		rows = 10
	}
	if m.sel < m.off {
		m.off = m.sel
	}
	if m.sel >= m.off+rows {
		m.off = m.sel - rows + 1
	}
	if m.off > max(0, len(m.items)-rows) {
		m.off = max(0, len(m.items)-rows)
	}
	if m.off < 0 {
		m.off = 0
	}
}

func (m *model) current() *Mention {
	if m.sel >= 0 && m.sel < len(m.items) {
		return m.items[m.sel]
	}
	return nil
}

func (m *model) Init() tea.Cmd {
	m.ticking = true
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

const (
	introDur = 1100 * time.Millisecond
	fadeDur  = 140 * time.Millisecond
)

func (m *model) animating() bool {
	t := time.Now()
	return t.Sub(m.start) < introDur+200*time.Millisecond || t.Sub(m.selAt) < fadeDur+20*time.Millisecond
}

func (m *model) ensureTick() tea.Cmd {
	if m.ticking || !m.animating() {
		return nil
	}
	m.ticking = true
	return tick()
}

func (m *model) moveTo(i int) {
	if len(m.items) == 0 {
		return
	}
	i = max(0, min(i, len(m.items)-1))
	if i != m.sel {
		m.sel = i
		m.selAt = time.Now()
		m.pvOff = 0
	}
	m.clamp()
}

func (m *model) setToast(s string, c RGB) {
	m.toast, m.toastColor, m.toastUntil = s, c, time.Now().Add(2200*time.Millisecond)
}

func (m *model) setGroup(i int) {
	i = max(0, min(i, len(groups)-1))
	if i == m.groupIdx {
		return
	}
	m.groupIdx = i
	m.resetSel = true
	m.selAt = time.Now()
	m.rebuild()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.computeLayout()
		m.clamp()
		return m, nil
	case tickMsg:
		m.now = time.Time(msg)
		if m.animating() || time.Now().Before(m.toastUntil) {
			return m, tick()
		}
		m.ticking = false
		return m, nil
	case OpenedMsg:
		if msg.Err != nil {
			m.setToast("Cannot open the link: "+msg.Err.Error(), cRed)
		}
		return m, nil
	case tea.KeyPressMsg:
		cmd := m.key(msg)
		return m, tea.Batch(cmd, m.ensureTick(), m.toastTick())
	case tea.MouseClickMsg:
		m.click(msg.Mouse())
		return m, tea.Batch(m.ensureTick())
	case tea.MouseWheelMsg:
		m.wheel(msg.Mouse())
		return m, m.ensureTick()
	}
	return m, nil
}

func (m *model) toastTick() tea.Cmd {
	if m.ticking || !time.Now().Before(m.toastUntil) {
		return nil
	}
	m.ticking = true
	return tick()
}

func (m *model) key(k tea.KeyPressMsg) tea.Cmd {
	s := k.String()
	if m.opt.NoQuit && (s == "q" || s == "ctrl+c") {
		return nil
	}
	if s == "ctrl+c" {
		return tea.Quit
	}
	if m.help {
		m.help = false
		return nil
	}
	if m.searching {
		switch s {
		case "esc":
			m.searching = false
			m.query = ""
			m.rebuild()
		case "enter", "down", "up", "tab":
			m.searching = false
			if s == "down" {
				m.moveTo(m.sel + 1)
			}
		case "backspace":
			if r := []rune(m.query); len(r) > 0 {
				m.query = string(r[:len(r)-1])
				m.resetSel = true
				m.rebuild()
			}
		case "ctrl+u":
			m.query = ""
			m.rebuild()
		default:
			if k.Text != "" {
				m.query += k.Text
				m.resetSel = true
				m.selAt = time.Now()
				m.rebuild()
			}
		}
		return nil
	}

	cur := m.current()
	switch s {
	case "q":
		return tea.Quit
	case "?":
		m.help = true
	case "esc":
		if m.query != "" {
			m.query = ""
			m.rebuild()
		}
	case "/":
		m.searching = true
		m.focus = focusList
	case "tab":
		m.focus = m.nextFocus(1)
	case "shift+tab":
		m.focus = m.nextFocus(-1)
	case "j", "down":
		m.moveDir(1)
	case "k", "up":
		m.moveDir(-1)
	case "g", "home":
		m.moveDir(-1 << 20)
	case "G", "end":
		m.moveDir(1 << 20)
	case "ctrl+d", "pgdown":
		m.moveDir(max(1, m.lay.listRows-1))
	case "ctrl+u", "pgup":
		m.moveDir(-max(1, m.lay.listRows-1))
	case "[":
		m.setGroup(m.groupIdx - 1)
	case "]":
		m.setGroup(m.groupIdx + 1)
	case "s":
		m.sortMode = (m.sortMode + 1) % len(sortNames)
		m.resetSel = true
		m.rebuild()
		m.setToast("Sort: "+sortNames[m.sortMode], cAccentHi)
	case "v":
		m.view = (m.view + 1) % len(viewNames)
		m.resetSel = true
		m.rebuild()
	case "h":
		m.showHidden = !m.showHidden
		m.rebuild()
		if m.showHidden {
			m.setToast("Showing noise, automation and own posts", cAmber)
		} else {
			m.setToast("Hiding noise, automation and own posts", cAccentHi)
		}
	case "o", "enter":
		if cur != nil {
			if m.focus == focusSide && s == "enter" {
				m.focus = focusList
				return nil
			}
			if m.opt.Open == nil {
				m.setToast("This host cannot open links. Press y to copy it.", cAmber)
				return nil
			}
			m.setToast("Opening "+truncate(cur.URL, 60), cAccentHi)
			return m.opt.Open(cur.URL)
		}
	case "y":
		if cur != nil {
			m.setToast("Copied the link", cGreen)
			if m.opt.Copy != nil {
				return m.opt.Copy(cur.URL)
			}
			return tea.SetClipboard(cur.URL)
		}
	case "f":
		m.triage(cur, "featured")
	case "x":
		m.triage(cur, "ignore")
	case "space":
		if cur != nil {
			m.triage(cur, "seen")
		}
	case "n":
		m.triage(cur, "new")
	}
	return nil
}

func (m *model) nextFocus(d int) int {
	order := []int{focusList}
	if m.lay.side.w > 0 {
		order = append([]int{focusSide}, order...)
	}
	if m.lay.preview.w > 0 {
		order = append(order, focusPreview)
	}
	idx := 0
	for i, f := range order {
		if f == m.focus {
			idx = i
		}
	}
	return order[(idx+d+len(order))%len(order)]
}

func (m *model) moveDir(d int) {
	switch m.focus {
	case focusSide:
		m.setGroup(m.groupIdx + max(-len(groups), min(d, len(groups))))
	case focusPreview:
		m.pvOff = max(0, m.pvOff+d)
	default:
		m.moveTo(m.sel + d)
	}
}

func (m *model) triage(cur *Mention, state string) {
	if cur == nil {
		return
	}
	prev := m.tri.Of(cur)
	if state == "featured" && prev == "featured" {
		state = "seen" // f again takes the star off
	}
	if err := m.tri.Set(cur, state); err != nil {
		m.setToast("Cannot save triage: "+err.Error(), cRed)
		return
	}
	m.computeStats()
	idx := m.sel
	m.rebuild()
	// Stay on the same index, so the next item slides under the cursor
	// when this one leaves the view.
	if !m.inView(cur, m.view) {
		m.sel = idx
		m.selAt = time.Now()
		m.clamp()
	} else if state == "seen" {
		m.moveTo(m.sel + 1)
	}
	switch state {
	case "featured":
		m.setToast(m.g.star+"  Featured", cAmber)
	case "ignore":
		m.setToast(m.g.ban+"  Ignored", cRed)
	case "seen":
		m.setToast(m.g.check+"  Seen", cBlue)
	case "new":
		m.setToast(m.g.dot+"  Marked new", cAccentHi)
	}
	if m.tri.Memory {
		m.toast += "  (not saved)"
	}
}

// OpenedMsg reports the result of an Open command.
type OpenedMsg struct{ Err error }

func (m *model) click(ms tea.Mouse) {
	l := m.lay
	if m.help {
		m.help = false
		return
	}
	switch {
	case l.side.has(ms.X, ms.Y):
		m.focus = focusSide
		for _, e := range l.sideRows {
			if e.y == ms.Y {
				m.setGroup(e.idx)
			}
		}
	case l.list.has(ms.X, ms.Y):
		m.focus = focusList
		for _, t := range l.tabs {
			if ms.Y == t.y && ms.X >= t.x0 && ms.X < t.x1 {
				if t.kind == 0 {
					m.view = t.idx
				} else {
					m.sortMode = (m.sortMode + 1) % len(sortNames)
				}
				m.resetSel = true
				m.rebuild()
				return
			}
		}
		if ms.Y >= l.rowsTop && l.rowH > 0 {
			i := m.off + (ms.Y-l.rowsTop)/l.rowH
			if i < len(m.items) && (ms.Y-l.rowsTop)/l.rowH < l.listRows {
				m.moveTo(i)
			}
		}
	case l.preview.has(ms.X, ms.Y):
		m.focus = focusPreview
		for _, t := range l.triBtns {
			if ms.Y == t.y && ms.X >= t.x0 && ms.X < t.x1 {
				m.triage(m.current(), []string{"new", "seen", "featured", "ignore"}[t.idx])
			}
		}
	}
}

func (m *model) wheel(ms tea.Mouse) {
	d := 0
	switch ms.Button {
	case tea.MouseWheelDown:
		d = 3
	case tea.MouseWheelUp:
		d = -3
	default:
		return
	}
	l := m.lay
	switch {
	case l.side.has(ms.X, ms.Y):
		m.setGroup(m.groupIdx + d/3)
	case l.preview.has(ms.X, ms.Y):
		m.pvOff = max(0, m.pvOff+d)
	default:
		m.moveTo(m.sel + d)
	}
}
