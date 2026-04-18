package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JordanLos/just-use-claude/internal/graph"
	"github.com/JordanLos/just-use-claude/internal/runner"
	"github.com/JordanLos/just-use-claude/internal/state"
	"github.com/spf13/cobra"
)

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "graph.yaml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no graph.yaml found (run juc init to create one)")
		}
		dir = parent
	}
}

func loadGraph(root string) (*graph.Graph, error) {
	g, err := graph.Load(filepath.Join(root, "graph.yaml"))
	if err != nil {
		return nil, err
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g, nil
}

var root = &cobra.Command{
	Use:   "juc",
	Short: "Just Use Claude — agentic build system",
}

var runCmd = &cobra.Command{
	Use:   "run [unit]",
	Short: "Run the graph or a single unit",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		g, err := loadGraph(root)
		if err != nil {
			return err
		}
		s, err := state.Load(root)
		if err != nil {
			return err
		}
		only := ""
		if len(args) == 1 {
			only = args[0]
			if _, ok := g.Units[only]; !ok {
				return fmt.Errorf("unknown unit: %q", only)
			}
		}
		r := runner.New(root, g, s)
		fmt.Printf("juc: running graph (%d units)\n", len(g.Units))
		return r.Run(only)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print current state of all units",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		g, err := loadGraph(root)
		if err != nil {
			return err
		}
		s, err := state.Load(root)
		if err != nil {
			return err
		}
		order := g.TopologicalOrder()
		icons := map[state.UnitState]string{
			state.Pending: "○",
			state.Running: "◉",
			state.Passed:  "✓",
			state.Failed:  "✗",
		}
		for _, id := range order {
			st := s.Get(id)
			icon := icons[st]
			deps := g.Units[id].Depends
			depStr := ""
			if len(deps) > 0 {
				depStr = fmt.Sprintf(" (needs: %s)", strings.Join(deps, ", "))
			}
			fmt.Printf("  %s %s%s\n", icon, id, depStr)
		}
		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate graph.yaml against the JUC schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		g, err := loadGraph(root)
		if err != nil {
			return fmt.Errorf("invalid: %w", err)
		}
		// Verify unit directories exist
		var missing []string
		for id := range g.Units {
			if _, err := os.Stat(filepath.Join(root, id, "agent.md")); err != nil {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return fmt.Errorf("units missing agent.md: %s", strings.Join(missing, ", "))
		}
		fmt.Printf("graph.yaml valid (%d units)\n", len(g.Units))
		return nil
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean [unit]",
	Short: "Reset state and output for all units or a single unit",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		g, err := loadGraph(root)
		if err != nil {
			return err
		}
		s, err := state.Load(root)
		if err != nil {
			return err
		}
		units := g.TopologicalOrder()
		if len(args) == 1 {
			units = []string{args[0]}
		}
		for _, id := range units {
			s.Units[id] = state.Pending
			os.RemoveAll(filepath.Join(root, id, "output"))
			os.MkdirAll(filepath.Join(root, id, "output"), 0755)
			fmt.Printf("  cleaned %s\n", id)
		}
		return s.Save()
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new JUC project in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		graphPath := filepath.Join(cwd, "graph.yaml")
		if _, err := os.Stat(graphPath); err == nil {
			return fmt.Errorf("graph.yaml already exists")
		}
		content := `juc: "2.0"

config:
  concurrency: 4

# Add units below. Example:
# research:
#   retries: 3
#
# implement:
#   depends: [research]
#   verify: [lint]
#   retries: infinite
`
		if err := os.WriteFile(graphPath, []byte(content), 0644); err != nil {
			return err
		}
		os.MkdirAll(filepath.Join(cwd, "checks"), 0755)
		fmt.Println("created graph.yaml")
		fmt.Println("created checks/")
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <unit>",
	Short: "Scaffold a new unit directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		id := args[0]
		dir := filepath.Join(root, id)
		if _, err := os.Stat(dir); err == nil {
			return fmt.Errorf("unit %q already exists", id)
		}
		os.MkdirAll(filepath.Join(dir, "context"), 0755)
		os.MkdirAll(filepath.Join(dir, "output"), 0755)
		agentContent := fmt.Sprintf(`---
model: claude-sonnet-4-6
tools: [Read, Grep, Glob]
---

# %s

Describe the task here.

## Output

Write results to the output/ directory.
`, id)
		if err := os.WriteFile(filepath.Join(dir, "agent.md"), []byte(agentContent), 0644); err != nil {
			return err
		}
		fmt.Printf("created %s/\n", id)
		fmt.Printf("  edit %s/agent.md to define the task\n", id)
		fmt.Printf("  add %q to graph.yaml\n", id)
		return nil
	},
}

func main() {
	root.AddCommand(runCmd, statusCmd, validateCmd, cleanCmd, initCmd, addCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
