package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGraph(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "graph.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "graph.yaml")
}

func TestLoad_valid(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
research:
  retries: 3
implement:
  depends: [research]
`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(g.Units))
	}
	if g.Units["implement"].Depends[0] != "research" {
		t.Errorf("expected implement to depend on research")
	}
}

func TestLoad_missingJUCField(t *testing.T) {
	path := writeGraph(t, `research:`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing juc field")
	}
}

func TestLoad_wrongVersion(t *testing.T) {
	path := writeGraph(t, `juc: "1.0"`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestLoad_defaults(t *testing.T) {
	path := writeGraph(t, `juc: "2.0"`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if g.Config.Concurrency != 4 {
		t.Errorf("expected default concurrency 4, got %d", g.Config.Concurrency)
	}
	if g.Config.Logging != "jsonl" {
		t.Errorf("expected default logging jsonl, got %s", g.Config.Logging)
	}
	if g.Config.Cache != "mtime" {
		t.Errorf("expected default cache mtime, got %s", g.Config.Cache)
	}
}

func TestLoad_unitDefaults(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
research:
`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	u := g.Units["research"]
	if u.Samples != 1 {
		t.Errorf("expected default samples 1, got %d", u.Samples)
	}
	if u.Consistency != "any" {
		t.Errorf("expected default consistency any, got %s", u.Consistency)
	}
}

func TestLoad_infiniteRetries(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
implement:
  retries: infinite
`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Units["implement"].Retries.Infinite {
		t.Error("expected infinite retries")
	}
}

func TestLoad_countedRetries(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
implement:
  retries: 5
`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	u := g.Units["implement"]
	if u.Retries.Infinite {
		t.Error("expected finite retries")
	}
	if u.Retries.Count != 5 {
		t.Errorf("expected retries count 5, got %d", u.Retries.Count)
	}
}

func TestValidate_cycle(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
a:
  depends: [b]
b:
  depends: [a]
`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Validate(); err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestValidate_unknownDep(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
implement:
  depends: [ghost]
`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestValidate_valid(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
a:
b:
  depends: [a]
c:
  depends: [b]
`)
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestTopologicalOrder(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
a:
b:
  depends: [a]
c:
  depends: [b]
`)
	g, _ := Load(path)
	order := g.TopologicalOrder()
	pos := func(id string) int {
		for i, v := range order {
			if v == id {
				return i
			}
		}
		return -1
	}
	if pos("a") >= pos("b") || pos("b") >= pos("c") {
		t.Errorf("wrong order: %v", order)
	}
}

func TestReady_noDeps(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
a:
b:
`)
	g, _ := Load(path)
	ready := g.Ready(map[string]bool{})
	if len(ready) != 2 {
		t.Errorf("expected 2 ready units, got %d", len(ready))
	}
}

func TestReady_depNotPassed(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
a:
b:
  depends: [a]
`)
	g, _ := Load(path)
	ready := g.Ready(map[string]bool{})
	if len(ready) != 1 || ready[0] != "a" {
		t.Errorf("expected only a to be ready, got %v", ready)
	}
}

func TestReady_depPassed(t *testing.T) {
	path := writeGraph(t, `
juc: "2.0"
a:
b:
  depends: [a]
`)
	g, _ := Load(path)
	ready := g.Ready(map[string]bool{"a": true})
	if len(ready) != 1 || ready[0] != "b" {
		t.Errorf("expected only b to be ready, got %v", ready)
	}
}
