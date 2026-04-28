package harness

import (
	"strings"
	"testing"
)

func TestParseFrontmatter_empty(t *testing.T) {
	fm := ParseFrontmatter([]byte("# just a body"))
	if fm.Harness != "" || fm.Model != "" || len(fm.Tools) != 0 {
		t.Errorf("expected empty frontmatter, got %+v", fm)
	}
}

func TestParseFrontmatter_full(t *testing.T) {
	fm := ParseFrontmatter([]byte("---\nharness: pi\nmodel: sonnet\ntools: read, write, bash\n---\nbody"))
	if fm.Harness != "pi" {
		t.Errorf("harness: want pi, got %q", fm.Harness)
	}
	if fm.Model != "sonnet" {
		t.Errorf("model: want sonnet, got %q", fm.Model)
	}
	if len(fm.Tools) != 3 || fm.Tools[0] != "read" || fm.Tools[2] != "bash" {
		t.Errorf("tools: want [read write bash], got %v", fm.Tools)
	}
}

func TestParseFrontmatter_toolsList(t *testing.T) {
	fm := ParseFrontmatter([]byte("---\ntools:\n  - read\n  - bash\n---\nbody"))
	if len(fm.Tools) != 2 || fm.Tools[1] != "bash" {
		t.Errorf("tools list: got %v", fm.Tools)
	}
}

func TestLoad_claudeBuiltin(t *testing.T) {
	cfg, err := Load("/nonexistent", "claude")
	if err != nil {
		t.Fatalf("Load(claude): %v", err)
	}
	if cfg.Binary != "claude" {
		t.Errorf("binary: want claude, got %q", cfg.Binary)
	}
	if cfg.Status != "stable" {
		t.Errorf("status: want stable, got %q", cfg.Status)
	}
}

func TestLoad_unknownHarness(t *testing.T) {
	_, err := Load("/nonexistent", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown harness")
	}
}

func TestResolveModel_tier(t *testing.T) {
	cfg, _ := Load("/nonexistent", "claude")
	got := cfg.ResolveModel("sonnet")
	if !strings.Contains(got, "claude-sonnet") {
		t.Errorf("ResolveModel(sonnet) = %q, want claude-sonnet-*", got)
	}
}

func TestResolveModel_passthrough(t *testing.T) {
	cfg, _ := Load("/nonexistent", "claude")
	id := "claude-custom-model-xyz"
	if got := cfg.ResolveModel(id); got != id {
		t.Errorf("passthrough: want %q, got %q", id, got)
	}
}

func TestResolveTools_translation(t *testing.T) {
	cfg, _ := Load("/nonexistent", "claude")
	got := cfg.ResolveTools([]string{"read", "bash", "edit"})
	want := []string{"Read", "Bash", "Edit"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: want %q, got %q", i, want[i], got[i])
		}
	}
}

func TestResolveTools_unsupported(t *testing.T) {
	cfg, _ := Load("/nonexistent", "goose")
	got := cfg.ResolveTools([]string{"glob"}) // goose has no glob
	if len(got) != 0 {
		t.Errorf("expected unsupported tool dropped, got %v", got)
	}
}

func TestBuildArgs_claude(t *testing.T) {
	cfg, _ := Load("/nonexistent", "claude")
	args := cfg.BuildArgs("/path/agent.md", "do the thing", "sonnet", []string{"read", "write"})

	has := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}
	if !has("--print") {
		t.Error("missing --print")
	}
	if !has("--agent") {
		t.Error("missing --agent")
	}
	if !has("--model") {
		t.Error("missing --model")
	}
	if !has("--allowedTools") {
		t.Error("missing --allowedTools")
	}
	if !has("do the thing") {
		t.Error("missing prompt body")
	}
}

func TestBuildArgs_noModelNoTools(t *testing.T) {
	cfg, _ := Load("/nonexistent", "claude")
	args := cfg.BuildArgs("/path/agent.md", "body", "", nil)
	for _, a := range args {
		if a == "--model" || a == "--allowedTools" {
			t.Errorf("unexpected flag %q when model/tools not set", a)
		}
	}
}
