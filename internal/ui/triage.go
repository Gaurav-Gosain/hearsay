package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unicode"

	"github.com/Gaurav-Gosain/hearsay/feed"
)

// Store keeps the triage state somewhere: a file, browser storage, or
// nowhere at all.
type Store interface {
	Load() (*feed.State, error)
	Save(*feed.State) error
}

// MemoryStore keeps the state in memory only.
type MemoryStore struct{}

// Load returns an empty state.
func (MemoryStore) Load() (*feed.State, error) { return feed.NewState(), nil }

// Save does nothing.
func (MemoryStore) Save(*feed.State) error { return nil }

// FileStore keeps the state in a JSON file, written atomically.
type FileStore struct{ Path string }

// Load reads the file. A missing file is an empty state.
func (f FileStore) Load() (*feed.State, error) {
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return feed.NewState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the state file: %w", err)
	}
	s, err := feed.ParseState(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Path, err)
	}
	return s, nil
}

// Save writes the file through a temporary file and a rename.
func (f FileStore) Save(s *feed.State) error {
	b, err := s.Encode()
	if err != nil {
		return err
	}
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, f.Path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Triage maps a mention to new, seen, featured or ignore. The view uses the
// word "ignore"; the state file says "ignored".
type Triage struct {
	store Store
	state *feed.State
	// Memory is true when nothing is saved, so the view can say so.
	Memory bool
}

// NewTriage loads the state from a store.
func NewTriage(st Store) (*Triage, error) {
	s, err := st.Load()
	if err != nil {
		return nil, err
	}
	_, mem := st.(MemoryStore)
	return &Triage{store: st, state: s, Memory: mem}, nil
}

// Of returns the state of a row.
func (t *Triage) Of(m *Mention) string {
	s := t.state.Get(m.ID)
	if s == "" {
		s = m.Initial
	}
	switch s {
	case "", feed.StateNew:
		return "new"
	case feed.StateIgnored:
		return "ignore"
	}
	return s
}

// Set changes a row and saves at once.
func (t *Triage) Set(m *Mention, s string) error {
	if s == "ignore" {
		s = feed.StateIgnored
	}
	t.state.Set(m.ID, s)
	return t.store.Save(t.state)
}

// fuzzy scores a lower-case query against a lower-case haystack. Every rune of
// the query must appear in order. Runs of adjacent runes and runes at word
// starts score higher. pos holds the matched indexes.
func fuzzy(hay, q []rune) (score int, pos []int, ok bool) {
	if len(q) == 0 {
		return 0, nil, true
	}
	// Prefer a contiguous match when there is one.
	if i := indexRunes(hay, q); i >= 0 {
		pos = make([]int, len(q))
		for k := range q {
			pos[k] = i + k
		}
		score = 100 + 10*len(q)
		if i == 0 || !isWordRune(hay[i-1]) {
			score += 40
		}
		return score - i/4, pos, true
	}
	qi, prev := 0, -2
	for hi := 0; hi < len(hay) && qi < len(q); hi++ {
		if hay[hi] != q[qi] {
			continue
		}
		s := 1
		if hi == prev+1 {
			s += 6
		}
		if hi == 0 || !isWordRune(hay[hi-1]) {
			s += 8
		}
		score += s
		pos = append(pos, hi)
		prev = hi
		qi++
	}
	if qi < len(q) {
		return 0, nil, false
	}
	// A query scattered in many pieces is noise: "devops" must not match
	// "dev-krish-xyz/OpenStore".
	runs := 1
	for i := 1; i < len(pos); i++ {
		if pos[i] != pos[i-1]+1 {
			runs++
		}
	}
	if runs > max(2, len(q)/3) {
		return 0, nil, false
	}
	// Penalise matches spread far apart.
	score -= (pos[len(pos)-1] - pos[0]) / 6
	return score, pos, true
}

func indexRunes(hay, q []rune) int {
outer:
	for i := 0; i+len(q) <= len(hay); i++ {
		for k := range q {
			if hay[i+k] != q[k] {
				continue outer
			}
		}
		return i
	}
	return -1
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
