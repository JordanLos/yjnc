package graph

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Concurrency int         `yaml:"concurrency"`
	Logging     string      `yaml:"logging"`
	Cache       string      `yaml:"cache"`
	Harness     string      `yaml:"harness"` // default harness for all units; empty = claude
	Secrets     []string    `yaml:"secrets"`
	Hooks       RunnerHooks `yaml:"hooks"`
}

type RunnerHooks struct {
	BeforeRun string `yaml:"before_run"`
	AfterRun  string `yaml:"after_run"`
	OnFailure string `yaml:"on_failure"`
}

type UnitHooks struct {
	Before    string `yaml:"before"`
	After     string `yaml:"after"`
	OnFailure string `yaml:"on_failure"`
}

type Retries struct {
	Infinite bool
	Count    int
}

func (r *Retries) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag == "!!str" && value.Value == "infinite" {
		r.Infinite = true
		return nil
	}
	return value.Decode(&r.Count)
}

type Unit struct {
	Depends     []string  `yaml:"depends"`
	Verify      []string  `yaml:"verify"`
	Retries     Retries   `yaml:"retries"`
	Samples     int       `yaml:"samples"`
	Consistency string    `yaml:"consistency"`
	Timeout     int       `yaml:"timeout"`
	Hooks       UnitHooks `yaml:"hooks"`
}

type Graph struct {
	JUC    string
	Config Config
	Checks map[string][]string
	Units  map[string]*Unit
}

var reservedKeys = map[string]bool{
	"juc":    true,
	"config": true,
	"checks": true,
}

func Load(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("%s is empty or invalid", path)
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a YAML mapping", path)
	}

	g := &Graph{
		Checks: make(map[string][]string),
		Units:  make(map[string]*Unit),
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i].Value
		val := root.Content[i+1]

		switch key {
		case "juc":
			g.JUC = val.Value
		case "config":
			if err := val.Decode(&g.Config); err != nil {
				return nil, fmt.Errorf("parsing config: %w", err)
			}
		case "checks":
			if err := val.Decode(&g.Checks); err != nil {
				return nil, fmt.Errorf("parsing checks: %w", err)
			}
		default:
			var u Unit
			if err := val.Decode(&u); err != nil {
				return nil, fmt.Errorf("parsing unit %q: %w", key, err)
			}
			g.Units[key] = &u
		}
	}

	if g.JUC == "" {
		return nil, fmt.Errorf("graph.yaml missing required field: juc")
	}
	if g.JUC != "2.0" {
		return nil, fmt.Errorf("unsupported juc version %q (expected 2.0)", g.JUC)
	}

	// Apply defaults
	if g.Config.Concurrency == 0 {
		g.Config.Concurrency = 4
	}
	if g.Config.Logging == "" {
		g.Config.Logging = "jsonl"
	}
	if g.Config.Cache == "" {
		g.Config.Cache = "mtime"
	}
	for _, u := range g.Units {
		if u.Samples == 0 {
			u.Samples = 1
		}
		if u.Consistency == "" {
			u.Consistency = "any"
		}
	}

	return g, nil
}

func (g *Graph) Validate() error {
	// Verify all depends reference known units
	for id, u := range g.Units {
		for _, dep := range u.Depends {
			if _, ok := g.Units[dep]; !ok {
				return fmt.Errorf("unit %q depends on unknown unit %q", id, dep)
			}
		}
	}
	// Cycle detection via DFS
	color := make(map[string]int) // 0=white 1=gray 2=black
	var visit func(id string) error
	visit = func(id string) error {
		if color[id] == 1 {
			return fmt.Errorf("cycle detected involving unit %q", id)
		}
		if color[id] == 2 {
			return nil
		}
		color[id] = 1
		for _, dep := range g.Units[id].Depends {
			if err := visit(dep); err != nil {
				return err
			}
		}
		color[id] = 2
		return nil
	}
	for id := range g.Units {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

// TopologicalOrder returns unit IDs in dependency order (deps before dependents).
func (g *Graph) TopologicalOrder() []string {
	visited := make(map[string]bool)
	var order []string
	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		for _, dep := range g.Units[id].Depends {
			visit(dep)
		}
		order = append(order, id)
	}
	for id := range g.Units {
		visit(id)
	}
	return order
}

// Ready returns unit IDs that have all dependencies in passed, excluding already-passed units.
func (g *Graph) Ready(passed map[string]bool) []string {
	var ready []string
	for id, u := range g.Units {
		if passed[id] {
			continue
		}
		allDone := true
		for _, dep := range u.Depends {
			if !passed[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, id)
		}
	}
	return ready
}
