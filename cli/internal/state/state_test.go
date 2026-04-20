package state

import (
	"testing"
)

func TestLoad_notExist(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Units) != 0 {
		t.Errorf("expected empty state, got %v", s.Units)
	}
}

func TestSave_roundtrip(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir)
	s.Units["research"] = Passed
	s.Units["implement"] = Failed
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	s2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Get("research") != Passed {
		t.Errorf("expected research=passed, got %s", s2.Get("research"))
	}
	if s2.Get("implement") != Failed {
		t.Errorf("expected implement=failed, got %s", s2.Get("implement"))
	}
}

func TestGet_missing(t *testing.T) {
	s, _ := Load(t.TempDir())
	if s.Get("unknown") != Pending {
		t.Errorf("expected pending for unknown unit")
	}
}

func TestSet_persists(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir)
	if err := s.Set("research", Running); err != nil {
		t.Fatal(err)
	}

	s2, _ := Load(dir)
	if s2.Get("research") != Running {
		t.Errorf("expected running, got %s", s2.Get("research"))
	}
}

func TestPassedSet(t *testing.T) {
	s, _ := Load(t.TempDir())
	s.Units["a"] = Passed
	s.Units["b"] = Failed
	s.Units["c"] = Passed

	passed := s.PassedSet()
	if !passed["a"] || !passed["c"] {
		t.Error("expected a and c in passed set")
	}
	if passed["b"] {
		t.Error("b should not be in passed set")
	}
}
