package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JordanLos/just-use-claude/internal/graph"
	"github.com/JordanLos/just-use-claude/internal/runner"
	"github.com/JordanLos/just-use-claude/internal/spec"
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
		if _, err := os.Stat(filepath.Join(cwd, "graph.yaml")); err == nil {
			return fmt.Errorf("graph.yaml already exists")
		}

		scaffold := []struct {
			path    string
			content string
			dir     bool
		}{
			{
				path: "graph.yaml",
				content: `juc: "2.0"

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
`,
			},
			{path: "checks/", dir: true},
			{path: ".claude/hooks/", dir: true},
			{
				path: ".claude/settings.json",
				content: `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "sh .claude/hooks/juc-log.sh"
          }
        ]
      }
    ]
  }
}
`,
			},
			{
				path: ".claude/hooks/juc-log.sh",
				content: `#!/bin/sh
# Appends Claude Code tool use events to .juc/logs/$JUC_UNIT/run-$JUC_RUN.jsonl
# JUC_UNIT and JUC_RUN are set by the juc runner.
[ -z "$JUC_UNIT" ] && exit 0
mkdir -p ".juc/logs/$JUC_UNIT"
cat >> ".juc/logs/$JUC_UNIT/run-${JUC_RUN:-1}.jsonl"
`,
			},
		}

		for _, s := range scaffold {
			if s.dir {
				os.MkdirAll(filepath.Join(cwd, s.path), 0755)
				fmt.Printf("  created %s\n", s.path)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(cwd, s.path)), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(cwd, s.path), []byte(s.content), 0644); err != nil {
				return err
			}
			fmt.Printf("  created %s\n", s.path)
		}
		if err := os.Chmod(filepath.Join(cwd, ".claude/hooks/juc-log.sh"), 0755); err != nil {
			return err
		}
		fmt.Println("\njuc project ready. Next:")
		fmt.Println("  juc add <unit>    scaffold your first unit")
		fmt.Println("  juc run           execute the graph")
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

var logsCmd = &cobra.Command{
	Use:   "logs <unit>",
	Short: "Display logs for a unit",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		unit := args[0]
		runFlag, _ := cmd.Flags().GetInt("run")
		allFlag, _ := cmd.Flags().GetBool("all")

		logsDir := filepath.Join(root, ".juc", "logs", unit)
		entries, err := os.ReadDir(logsDir)
		if os.IsNotExist(err) {
			return fmt.Errorf("no logs found for unit %q", unit)
		}
		if err != nil {
			return err
		}

		var runs []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "run-") && strings.HasSuffix(e.Name(), ".jsonl") {
				runs = append(runs, e.Name())
			}
		}
		sort.Strings(runs)
		if len(runs) == 0 {
			return fmt.Errorf("no runs found for unit %q", unit)
		}

		var toShow []string
		switch {
		case allFlag:
			toShow = runs
		case runFlag > 0:
			name := fmt.Sprintf("run-%d.jsonl", runFlag)
			if _, err := os.Stat(filepath.Join(logsDir, name)); err != nil {
				return fmt.Errorf("run %d not found for unit %q", runFlag, unit)
			}
			toShow = []string{name}
		default:
			toShow = []string{runs[len(runs)-1]}
		}

		for _, run := range toShow {
			fmt.Printf("\n%s/%s\n", unit, run)
			if err := printLog(filepath.Join(logsDir, run)); err != nil {
				return err
			}
		}
		return nil
	},
}

type logEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Unit      string `json:"unit"`
	Message   string `json:"message"`
	Check     string `json:"check"`
	ExitCode  *int   `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Tool      string `json:"tool"`
}

func printLog(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e logEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			fmt.Printf("  [raw] %s\n", line)
			continue
		}
		ts := ""
		if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			ts = t.Local().Format("15:04:05")
		}
		switch e.Type {
		case "agent_start":
			fmt.Printf("  %s  → agent started\n", ts)
		case "agent_end":
			fmt.Printf("  %s  ✓ agent completed\n", ts)
		case "check_result":
			if e.ExitCode != nil && *e.ExitCode == 0 {
				fmt.Printf("  %s  ✓ check %-12s exit 0\n", ts, e.Check)
			} else {
				code := -1
				if e.ExitCode != nil {
					code = *e.ExitCode
				}
				fmt.Printf("  %s  ✗ check %-12s exit %d\n", ts, e.Check, code)
				if e.Stdout != "" {
					fmt.Printf("    stdout: %s\n", strings.TrimSpace(e.Stdout))
				}
				if e.Stderr != "" {
					fmt.Printf("    stderr: %s\n", strings.TrimSpace(e.Stderr))
				}
			}
		case "event":
			fmt.Printf("  %s  ! %s\n", ts, e.Message)
		case "tool_call":
			fmt.Printf("  %s  [%s]\n", ts, e.Tool)
		default:
			fmt.Printf("  %s  %s\n", ts, e.Type)
		}
	}
	return nil
}

func init() {
	logsCmd.Flags().Int("run", 0, "show a specific run number")
	logsCmd.Flags().Bool("all", false, "show all runs")
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install juc into Claude Code globally (~/.claude)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := writeJucSkill(); err != nil {
			return err
		}
		if err := patchClaudeMD(); err != nil {
			return err
		}
		home, _ := os.UserHomeDir()
		fmt.Printf("juc installed into Claude Code.\n")
		fmt.Printf("  skill:  %s/.claude/skills/juc-spec.md\n", home)
		fmt.Printf("  claude: %s/.claude/CLAUDE.md\n", home)
		fmt.Println("\nIn any Claude Code session, invoke /juc-spec to load the specification.")
		return nil
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Update the juc Claude Code skill to the current version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := writeJucSkill(); err != nil {
			return err
		}
		home, _ := os.UserHomeDir()
		fmt.Printf("juc skill updated: %s/.claude/skills/juc-spec.md\n", home)
		return nil
	},
}

func writeJucSkill() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}
	content := "---\ndescription: Full JUC 2.0 specification and CLI reference. Read this before creating, modifying, or running any juc project.\n---\n\n" + string(spec.Content)
	return os.WriteFile(filepath.Join(skillsDir, "juc-spec.md"), []byte(content), 0644)
}

func patchClaudeMD() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")

	existing, err := os.ReadFile(claudeMD)
	if err == nil && strings.Contains(string(existing), "## juc") {
		fmt.Println("  (CLAUDE.md already contains juc entry, skipping)")
		return nil
	}

	stub := "\n## juc\n\njuc (Just Use Claude) is installed. Before working with any juc project, invoke the `/juc-spec` skill to load the full specification and CLI reference.\n"
	f, err := os.OpenFile(claudeMD, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(stub)
	return err
}

func main() {
	root.AddCommand(runCmd, statusCmd, validateCmd, cleanCmd, initCmd, addCmd, logsCmd, installCmd, upgradeCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
