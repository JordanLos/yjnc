package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/JordanLos/just-use-claude/internal/graph"
	"github.com/JordanLos/just-use-claude/internal/logger"
	"github.com/JordanLos/just-use-claude/internal/state"
)

// Agent executes a unit's agent.md. Injected so the runner is testable.
type Agent interface {
	Execute(root, id, agentPath string, attempt int) error
}

// CLIAgent invokes the claude CLI.
type CLIAgent struct{}

func (c *CLIAgent) Execute(root, id, agentPath string, attempt int) error {
	cmd := exec.Command("claude", "--agent", agentPath)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"JUC_UNIT="+id,
		fmt.Sprintf("JUC_RUN=%d", attempt),
	)
	return cmd.Run()
}

type Runner struct {
	Root  string
	Graph *graph.Graph
	State *state.State
	Agent Agent
}

func New(root string, g *graph.Graph, s *state.State) *Runner {
	return &Runner{Root: root, Graph: g, State: s, Agent: &CLIAgent{}}
}

func (r *Runner) Run(only string) error {
	if err := r.runHook(r.Graph.Config.Hooks.BeforeRun, ""); err != nil {
		return fmt.Errorf("before_run hook: %w", err)
	}

	passed := r.State.PassedSet()
	inflight := make(map[string]bool)
	mu := sync.Mutex{}
	sem := make(chan struct{}, r.Graph.Config.Concurrency)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	for {
		mu.Lock()
		ready := r.Graph.Ready(passed)
		var toRun []string
		for _, id := range ready {
			if only != "" && id != only {
				continue
			}
			if !inflight[id] {
				toRun = append(toRun, id)
				inflight[id] = true
			}
		}
		allDone := len(passed) == len(r.Graph.Units)
		mu.Unlock()

		if allDone {
			break
		}
		if len(toRun) == 0 {
			wg.Wait()
			select {
			case err := <-errCh:
				return err
			default:
			}
			mu.Lock()
			if len(passed) == len(r.Graph.Units) {
				mu.Unlock()
				break
			}
			mu.Unlock()
			break
		}

		for _, id := range toRun {
			id := id
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				if err := r.runUnit(id); err != nil {
					r.runHook(r.Graph.Config.Hooks.OnFailure, id)
					select {
					case errCh <- fmt.Errorf("unit %q failed: %w", id, err):
					default:
					}
					return
				}
				mu.Lock()
				passed[id] = true
				mu.Unlock()
			}()
		}

		wg.Wait()
		select {
		case err := <-errCh:
			return err
		default:
		}
	}

	r.runHook(r.Graph.Config.Hooks.AfterRun, "")
	return nil
}

func (r *Runner) runUnit(id string) error {
	u := r.Graph.Units[id]

	if err := r.State.Set(id, state.Running); err != nil {
		return err
	}
	if err := r.stageContext(id); err != nil {
		return fmt.Errorf("staging context: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(r.Root, id, "output"), 0755); err != nil {
		return err
	}

	r.runHook(u.Hooks.Before, id)

	attempt := 0
	for {
		attempt++
		log, err := logger.New(r.Root, id, attempt)
		if err != nil {
			return err
		}

		fmt.Printf("  → %s (attempt %d)\n", id, attempt)
		log.AgentStart(id)

		agentPath := filepath.Join(r.Root, id, "agent.md")
		if err := r.Agent.Execute(r.Root, id, agentPath, attempt); err != nil {
			log.Event(id, fmt.Sprintf("agent error: %v", err))
			log.Close()
		} else {
			log.AgentEnd(id)
			checkErr := r.runChecks(id, log)
			log.Close()

			if checkErr == nil {
				r.State.Set(id, state.Passed)
				r.runHook(u.Hooks.After, id)
				fmt.Printf("  ✓ %s\n", id)
				return nil
			}
			fmt.Printf("  ✗ %s: %v\n", id, checkErr)
		}

		if !u.Retries.Infinite && attempt > u.Retries.Count {
			r.State.Set(id, state.Failed)
			r.runHook(u.Hooks.OnFailure, id)
			return fmt.Errorf("checks failed after %d attempt(s)", attempt)
		}
		fmt.Printf("  ↺ %s retrying...\n", id)
	}
}

func (r *Runner) runChecks(id string, log *logger.Logger) error {
	u := r.Graph.Units[id]

	if len(u.Verify) == 0 {
		outputDir := filepath.Join(r.Root, id, "output")
		entries, err := os.ReadDir(outputDir)
		if err != nil || len(entries) == 0 {
			return fmt.Errorf("default check failed: output/ is empty")
		}
		return nil
	}

	for _, check := range u.Verify {
		path := r.resolveCheck(check)
		stdout, stderr, exitCode, err := r.execCheck(path, filepath.Join(r.Root, id, "output"))
		log.CheckResult(id, check, exitCode, stdout, stderr)
		if err != nil || exitCode != 0 {
			return fmt.Errorf("check %q failed (exit %d)", check, exitCode)
		}
	}
	return nil
}

func (r *Runner) resolveCheck(check string) string {
	if strings.HasPrefix(check, "./") || strings.HasPrefix(check, "/") {
		return check
	}
	return filepath.Join(r.Root, "checks", check+".sh")
}

func (r *Runner) execCheck(path, outputDir string) (stdout, stderr string, exitCode int, err error) {
	cmd := exec.Command(path, outputDir)
	cmd.Dir = r.Root
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		return stdout, stderr, 1, runErr
	}
	return stdout, stderr, 0, nil
}

func (r *Runner) stageContext(id string) error {
	u := r.Graph.Units[id]
	if len(u.Depends) == 0 {
		return nil
	}
	destDir := filepath.Join(r.Root, id, "context")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	for _, dep := range u.Depends {
		srcDir := filepath.Join(r.Root, dep, "output")
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			src := filepath.Join(srcDir, e.Name())
			dst := filepath.Join(destDir, dep+"-"+e.Name())
			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Runner) runHook(hook, unit string) error {
	if hook == "" {
		return nil
	}
	args := []string{"-c", hook}
	if unit != "" {
		args = append(args, "--", unit)
	}
	cmd := exec.Command("sh", args...)
	cmd.Dir = r.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
