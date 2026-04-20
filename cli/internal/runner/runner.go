package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/JordanLos/just-use-claude/internal/graph"
	"github.com/JordanLos/just-use-claude/internal/logger"
	"github.com/JordanLos/just-use-claude/internal/state"
)

// Agent executes a unit's agent.md. Injected so the runner is testable.
type Agent interface {
	Execute(ctx context.Context, root, id, agentPath string, attempt int) error
}

// CLIAgent invokes the claude CLI.
type CLIAgent struct{}

func (c *CLIAgent) Execute(ctx context.Context, root, id, agentPath string, attempt int) error {
	cmd := exec.CommandContext(ctx, "claude", "--agent", agentPath)
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

	if r.isCached(id) {
		fmt.Printf("  ⟳ %s (cached)\n", id)
		return r.State.Set(id, state.Passed)
	}

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

	var err error
	if u.Samples > 1 {
		err = r.runWithSampling(id)
	} else {
		err = r.runWithRetries(id)
	}

	if err != nil {
		r.State.Set(id, state.Failed)
		r.runHook(u.Hooks.OnFailure, id)
		return err
	}

	r.State.Set(id, state.Passed)
	r.runHook(u.Hooks.After, id)
	if r.Graph.Config.Cache == "content-addressed" {
		r.saveContentHash(id)
	}
	fmt.Printf("  ✓ %s\n", id)
	return nil
}

func (r *Runner) runWithRetries(id string) error {
	u := r.Graph.Units[id]
	attempt := 0
	for {
		attempt++
		fmt.Printf("  → %s (attempt %d)\n", id, attempt)

		if err := r.runOnce(id, "", attempt); err == nil {
			return nil
		} else if !u.Retries.Infinite && attempt > u.Retries.Count {
			return fmt.Errorf("checks failed after %d attempt(s)", attempt)
		}
		fmt.Printf("  ↺ %s retrying...\n", id)
	}
}

func (r *Runner) runWithSampling(id string) error {
	u := r.Graph.Units[id]

	results := make([]error, u.Samples)
	var wg sync.WaitGroup
	sem := make(chan struct{}, r.Graph.Config.Concurrency)

	for n := 1; n <= u.Samples; n++ {
		n := n
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fmt.Printf("  → %s (sample %d/%d)\n", id, n, u.Samples)
			results[n-1] = r.runOnce(id, fmt.Sprintf("sample-%d", n), n)
		}()
	}
	wg.Wait()

	passed := 0
	for _, err := range results {
		if err == nil {
			passed++
		}
	}

	switch u.Consistency {
	case "any":
		for n, err := range results {
			if err == nil {
				return r.adoptSample(id, n+1)
			}
		}
		return fmt.Errorf("no passing samples (0/%d)", u.Samples)
	case "majority":
		if passed > u.Samples/2 {
			for n, err := range results {
				if err == nil {
					return r.adoptSample(id, n+1)
				}
			}
		}
		return fmt.Errorf("majority consistency failed (%d/%d passed)", passed, u.Samples)
	case "all":
		if passed == u.Samples {
			return r.adoptSample(id, 1)
		}
		return fmt.Errorf("all consistency failed (%d/%d passed)", passed, u.Samples)
	default:
		return r.runCustomConsistency(id, u.Consistency, u.Samples)
	}
}

func (r *Runner) runOnce(id, outputSubdir string, attempt int) error {
	u := r.Graph.Units[id]

	outputDir := filepath.Join(r.Root, id, "output")
	if outputSubdir != "" {
		outputDir = filepath.Join(outputDir, outputSubdir)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
	}

	logUnit := id
	if outputSubdir != "" {
		logUnit = id + "/" + outputSubdir
	}
	log, err := logger.New(r.Root, logUnit, attempt)
	if err != nil {
		return err
	}
	defer log.Close()

	ctx := context.Background()
	if u.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(u.Timeout)*time.Second)
		defer cancel()
	}

	log.AgentStart(id)
	agentPath := filepath.Join(r.Root, id, "agent.md")
	if err := r.Agent.Execute(ctx, r.Root, id, agentPath, attempt); err != nil {
		log.Event(id, fmt.Sprintf("agent error: %v", err))
		return err
	}
	log.AgentEnd(id)

	return r.runChecksInDir(id, outputDir, log)
}

func (r *Runner) adoptSample(id string, n int) error {
	sampleDir := filepath.Join(r.Root, id, "output", fmt.Sprintf("sample-%d", n))
	outputDir := filepath.Join(r.Root, id, "output")
	entries, err := os.ReadDir(sampleDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sampleDir, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outputDir, e.Name()), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) runCustomConsistency(id, scriptPath string, samples int) error {
	args := []string{id}
	for n := 1; n <= samples; n++ {
		args = append(args, filepath.Join(r.Root, id, "output", fmt.Sprintf("sample-%d", n)))
	}
	cmd := exec.Command(scriptPath, args...)
	cmd.Dir = r.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *Runner) runChecksInDir(id, outputDir string, log *logger.Logger) error {
	u := r.Graph.Units[id]

	if len(u.Verify) == 0 {
		entries, err := os.ReadDir(outputDir)
		if err != nil || len(entries) == 0 {
			return fmt.Errorf("default check failed: output/ is empty")
		}
		return nil
	}

	for _, check := range u.Verify {
		path := r.resolveCheck(check)
		stdout, stderr, exitCode, err := r.execCheck(path, outputDir)
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

// --- caching ---

func (r *Runner) isCached(id string) bool {
	switch r.Graph.Config.Cache {
	case "none":
		return false
	case "content-addressed":
		return r.isCachedContentAddressed(id)
	default: // "mtime"
		return r.isCachedMtime(id)
	}
}

func (r *Runner) isCachedMtime(id string) bool {
	outputDir := filepath.Join(r.Root, id, "output")
	outInfo, err := os.Stat(outputDir)
	if err != nil {
		return false
	}
	outTime := outInfo.ModTime()

	if info, err := os.Stat(filepath.Join(r.Root, id, "agent.md")); err != nil || info.ModTime().After(outTime) {
		return false
	}

	contextDir := filepath.Join(r.Root, id, "context")
	if entries, err := os.ReadDir(contextDir); err == nil {
		for _, e := range entries {
			if info, err := e.Info(); err != nil || info.ModTime().After(outTime) {
				return false
			}
		}
	}

	entries, err := os.ReadDir(outputDir)
	return err == nil && len(entries) > 0
}

func (r *Runner) isCachedContentAddressed(id string) bool {
	hash, err := r.computeInputHash(id)
	if err != nil {
		return false
	}
	stored, err := os.ReadFile(filepath.Join(r.Root, ".juc", "cache", id+".hash"))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(stored)) == hash
}

func (r *Runner) saveContentHash(id string) {
	hash, err := r.computeInputHash(id)
	if err != nil {
		return
	}
	dir := filepath.Join(r.Root, ".juc", "cache")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, id+".hash"), []byte(hash), 0644)
}

func (r *Runner) computeInputHash(id string) (string, error) {
	h := sha256.New()

	data, err := os.ReadFile(filepath.Join(r.Root, id, "agent.md"))
	if err != nil {
		return "", err
	}
	h.Write(data)

	contextDir := filepath.Join(r.Root, id, "context")
	entries, _ := os.ReadDir(contextDir)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(contextDir, e.Name()))
		if err != nil {
			continue
		}
		h.Write([]byte(e.Name()))
		h.Write(data)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
