package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type UnitState string

const (
	Pending UnitState = "pending"
	Running UnitState = "running"
	Passed  UnitState = "passed"
	Failed  UnitState = "failed"
)

type State struct {
	Units map[string]UnitState `json:"units"`
	path  string
}

func Load(root string) (*State, error) {
	path := filepath.Join(root, ".juc", "state.json")
	s := &State{Units: make(map[string]UnitState), path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	s.path = path
	return s, nil
}

func (s *State) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *State) Set(unit string, st UnitState) error {
	s.Units[unit] = st
	return s.Save()
}

func (s *State) Get(unit string) UnitState {
	if st, ok := s.Units[unit]; ok {
		return st
	}
	return Pending
}

func (s *State) PassedSet() map[string]bool {
	m := make(map[string]bool)
	for id, st := range s.Units {
		if st == Passed {
			m[id] = true
		}
	}
	return m
}
