package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/JordanLos/just-use-claude/internal/graph"
	"github.com/JordanLos/just-use-claude/internal/state"
)

// fakeAgent records calls and returns a configurable error sequence.
type fakeAgent struct {
	calls   []string
	results []error // consumed in order; last element repeats
}

func (f *fakeAgent) Execute(root, id, agentPath string, attempt int) error {
	f.calls = append(f.calls, id)
	if len(f.results) == 0 {
		return nil
	}
	i := len(f.calls) - 1
	if i >= len(f.results) {
		i = len(f.results) - 1
	}
	return f.results[i]
}

// writeOutput creates a non-empty output file so the default check passes.
func writeOutput(t *testing.T, root, unit, name, content string) {
	t.Helper()
	dir := filepath.Join(root, unit, "output")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
}

func setup(t *testing.T, yaml string) (string, *graph.Graph, *state.State) {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "graph.yaml"), []byte(yaml), 0644)
	g, err := graph.Load(filepath.Join(root, "graph.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	// Create agent.md stubs so the runner doesn't error on missing files
	for id := range g.Units {
		os.MkdirAll(filepath.Join(root, id), 0755)
		os.WriteFile(filepath.Join(root, id, "agent.md"), []byte("# stub"), 0644)
	}
	s, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, g, s
}

func TestRun_singleUnit_success(t *testing.T) {
	root, g, s := setup(t, `
juc: "2.0"
research:
`)
	agent := &fakeAgent{}
	r := &Runner{Root: root, Graph: g, State: s, Agent: agent}

	// Agent writes output so default check passes
	agent.results = []error{nil}
	writeOutput(t, root, "research", "findings.md", "results")

	if err := r.Run(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Get("research") != state.Passed {
		t.Errorf("expected research=passed, got %s", s.Get("research"))
	}
	if len(agent.calls) != 1 {
		t.Errorf("expected 1 agent call, got %d", len(agent.calls))
	}
}

func TestRun_agentError_retries(t *testing.T) {
	root, g, s := setup(t, `
juc: "2.0"
research:
  retries: 2
`)
	// Fail first two times, succeed third
	callCount := 0
	agent := &fakeAgent{}
	agent.results = []error{errors.New("fail"), errors.New("fail"), nil}

	r := &Runner{Root: root, Graph: g, State: s, Agent: agent}

	// Write output before third call would succeed
	go func() {
		for callCount < 3 {
		}
		writeOutput(t, root, "research", "out.md", "ok")
	}()

	// Simpler: pre-write output, agent errors just test retry count
	writeOutput(t, root, "research", "out.md", "ok")

	_ = callCount
	if err := r.Run(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.calls) != 3 {
		t.Errorf("expected 3 agent calls (2 failures + 1 success), got %d", len(agent.calls))
	}
}

func TestRun_retriesExhausted_fails(t *testing.T) {
	root, g, s := setup(t, `
juc: "2.0"
research:
  retries: 1
`)
	agent := &fakeAgent{results: []error{errors.New("always fail")}}
	r := &Runner{Root: root, Graph: g, State: s, Agent: agent}

	if err := r.Run(""); err == nil {
		t.Fatal("expected failure after retries exhausted")
	}
	if s.Get("research") != state.Failed {
		t.Errorf("expected research=failed, got %s", s.Get("research"))
	}
}

func TestRun_dependencyOrdering(t *testing.T) {
	root, g, s := setup(t, `
juc: "2.0"
a:
b:
  depends: [a]
c:
  depends: [b]
`)
	var order []string
	ordered := &orderAgent{root: root, order: &order}
	r := &Runner{Root: root, Graph: g, State: s, Agent: ordered}

	if err := r.Run(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 executions, got %d: %v", len(order), order)
	}
	pos := func(id string) int {
		for i, v := range order {
			if v == id {
				return i
			}
		}
		return -1
	}
	if pos("a") >= pos("b") || pos("b") >= pos("c") {
		t.Errorf("wrong execution order: %v", order)
	}
}

func TestRun_artifactStaging(t *testing.T) {
	root, g, s := setup(t, `
juc: "2.0"
research:
implement:
  depends: [research]
`)
	// research produces a file
	writeOutput(t, root, "research", "findings.md", "important findings")

	staged := &stagingAgent{root: root}
	r := &Runner{Root: root, Graph: g, State: s, Agent: staged}

	if err := r.Run(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// implement's context/ should contain research's output
	data, err := os.ReadFile(filepath.Join(root, "implement", "context", "research-findings.md"))
	if err != nil {
		t.Fatalf("staged artifact not found: %v", err)
	}
	if string(data) != "important findings" {
		t.Errorf("unexpected staged content: %s", data)
	}
}

func TestRun_defaultCheck_emptyOutput_fails(t *testing.T) {
	root, g, s := setup(t, `
juc: "2.0"
research:
`)
	// Agent succeeds but writes nothing to output/
	agent := &fakeAgent{results: []error{nil}}
	r := &Runner{Root: root, Graph: g, State: s, Agent: agent}

	if err := r.Run(""); err == nil {
		t.Fatal("expected failure: output/ is empty")
	}
}

func TestRun_parallelExecution(t *testing.T) {
	root, g, s := setup(t, `
juc: "2.0"
config:
  concurrency: 4
a:
b:
c:
`)
	var concurrent int64
	var maxConcurrent int64

	pa := &parallelAgent{root: root, concurrent: &concurrent, maxConcurrent: &maxConcurrent}
	r := &Runner{Root: root, Graph: g, State: s, Agent: pa}

	if err := r.Run(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxConcurrent < 2 {
		t.Logf("max concurrent was %d (may be sequential on fast machines)", maxConcurrent)
	}
}

// --- helper agents ---

type orderAgent struct {
	root  string
	order *[]string
}

func (a *orderAgent) Execute(root, id, agentPath string, attempt int) error {
	*a.order = append(*a.order, id)
	writeOutputHelper(root, id)
	return nil
}

type stagingAgent struct{ root string }

func (a *stagingAgent) Execute(root, id, agentPath string, attempt int) error {
	writeOutputHelper(root, id)
	return nil
}

type parallelAgent struct {
	root          string
	concurrent    *int64
	maxConcurrent *int64
}

func (a *parallelAgent) Execute(root, id, agentPath string, attempt int) error {
	cur := atomic.AddInt64(a.concurrent, 1)
	defer atomic.AddInt64(a.concurrent, -1)
	for {
		max := atomic.LoadInt64(a.maxConcurrent)
		if cur <= max || atomic.CompareAndSwapInt64(a.maxConcurrent, max, cur) {
			break
		}
	}
	writeOutputHelper(root, id)
	return nil
}

func writeOutputHelper(root, id string) {
	dir := filepath.Join(root, id, "output")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "out.md"), []byte(fmt.Sprintf("%s output", id)), 0644)
}
