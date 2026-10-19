package feed

import (
	"encoding/json"
	"fmt"
	"time"
)

// State is the local triage file: item id to its state.
type State struct {
	Version int                  `json:"version"`
	Items   map[string]StateItem `json:"items"`
}

// StateItem is the triage of one item.
type StateItem struct {
	State string `json:"state"`
	At    string `json:"at,omitempty"`
}

// NewState returns an empty state.
func NewState() *State { return &State{Version: Version, Items: map[string]StateItem{}} }

// ParseState decodes a state file. It also reads the older flat form, a map
// of id to state, and the old word "ignore".
func ParseState(b []byte) (*State, error) {
	s := NewState()
	if len(b) == 0 {
		return s, nil
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, fmt.Errorf("the state file is not valid JSON: %w", err)
	}
	if _, ok := probe["items"]; ok {
		if err := json.Unmarshal(b, s); err != nil {
			return nil, fmt.Errorf("the state file is not valid: %w", err)
		}
		if s.Items == nil {
			s.Items = map[string]StateItem{}
		}
	} else {
		for id, raw := range probe {
			var v string
			if json.Unmarshal(raw, &v) == nil {
				s.Items[id] = StateItem{State: v}
			}
		}
	}
	for id, it := range s.Items {
		if it.State == "ignore" {
			it.State = StateIgnored
			s.Items[id] = it
		}
	}
	s.Version = Version
	return s, nil
}

// Get returns the state of an id, or "" when the file has none.
func (s *State) Get(id string) string { return s.Items[id].State }

// Set records a state for an id, with the time.
func (s *State) Set(id, state string) {
	s.Items[id] = StateItem{State: state, At: time.Now().UTC().Format(time.RFC3339)}
}

// Encode writes the state as indented JSON with a final newline.
func (s *State) Encode() ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
