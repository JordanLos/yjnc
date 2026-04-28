package harness

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed claude.yaml forge.yaml goose.yaml pi.yaml opencode.yaml
var builtins embed.FS

// Config defines how to invoke a harness — binary, flags, model/tool translation.
// Harness YAML files are the durable artifacts; the runner implementation is secondary.
type Config struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Source  string `yaml:"source"`
	Status  string `yaml:"status"` // stable | experimental

	Binary    string   `yaml:"binary"`
	FixedArgs []string `yaml:"fixed_args"`

	// AgentFlag: receives the agent.md file path as the system-prompt definition.
	// Empty = harness has no separate system-prompt file mechanism.
	AgentFlag string `yaml:"agent_flag"`

	// PromptFlag: flag for the task/prompt text. Empty = positional argument.
	PromptFlag  string `yaml:"prompt_flag"`
	PromptStdin bool   `yaml:"prompt_stdin"`

	ModelFlag string            `yaml:"model_flag"`
	ModelMap  map[string]string `yaml:"model_map"`

	ToolsFlag      string            `yaml:"tools_flag"`
	ToolsSeparator string            `yaml:"tools_separator"`
	ToolMap        map[string]string `yaml:"tool_map"`
}

// Frontmatter holds harness-relevant fields parsed from an agent.md YAML header.
type Frontmatter struct {
	Harness string
	Model   string
	Tools   []string
}

// ParseFrontmatter extracts harness/model/tools from a leading --- YAML block.
// Returns a zero Frontmatter (no error) when no front matter is present.
func ParseFrontmatter(data []byte) Frontmatter {
	s := string(data)
	if !strings.HasPrefix(s, "---") {
		return Frontmatter{}
	}
	rest := s[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return Frontmatter{}
	}

	var raw struct {
		Harness string    `yaml:"harness"`
		Model   string    `yaml:"model"`
		Tools   yaml.Node `yaml:"tools"`
	}
	if err := yaml.Unmarshal([]byte(rest[:idx]), &raw); err != nil {
		return Frontmatter{}
	}

	fm := Frontmatter{Harness: raw.Harness, Model: raw.Model}

	switch raw.Tools.Kind {
	case yaml.ScalarNode:
		for _, t := range strings.FieldsFunc(raw.Tools.Value, func(r rune) bool {
			return r == ',' || r == ' '
		}) {
			if t = strings.TrimSpace(t); t != "" {
				fm.Tools = append(fm.Tools, t)
			}
		}
	case yaml.SequenceNode:
		for _, n := range raw.Tools.Content {
			if n.Value != "" {
				fm.Tools = append(fm.Tools, n.Value)
			}
		}
	}

	return fm
}

// Load returns the named harness Config. Checks <root>/.juc/harnesses/<name>.yaml
// first (user override), then falls back to embedded built-ins.
func Load(root, name string) (*Config, error) {
	if name == "" {
		name = "claude"
	}
	if data, err := os.ReadFile(filepath.Join(root, ".juc", "harnesses", name+".yaml")); err == nil {
		return parse(data)
	}
	data, err := builtins.ReadFile(name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("unknown harness %q: no built-in or .juc/harnesses/%s.yaml found", name, name)
	}
	return parse(data)
}

func parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.ToolsSeparator == "" {
		c.ToolsSeparator = ","
	}
	return &c, nil
}

// ResolveModel maps a tier name (haiku/sonnet/opus) to a concrete model ID,
// or passes a full model ID through unchanged.
func (c *Config) ResolveModel(model string) string {
	if model == "" {
		return ""
	}
	if v, ok := c.ModelMap[model]; ok {
		return v
	}
	return model
}

// ResolveTools translates canonical tool names to harness-specific names.
// Tools with no mapping or an empty value are silently dropped.
func (c *Config) ResolveTools(tools []string) []string {
	var out []string
	for _, t := range tools {
		if mapped, ok := c.ToolMap[strings.ToLower(t)]; ok && mapped != "" {
			out = append(out, mapped)
		}
	}
	return out
}

// BuildArgs constructs the complete argument list for one agent invocation.
func (c *Config) BuildArgs(agentPath, body, model string, tools []string) []string {
	args := append([]string(nil), c.FixedArgs...)

	if c.AgentFlag != "" {
		args = append(args, c.AgentFlag, agentPath)
	}

	if resolved := c.ResolveModel(model); c.ModelFlag != "" && resolved != "" {
		args = append(args, c.ModelFlag, resolved)
	}

	if mapped := c.ResolveTools(tools); c.ToolsFlag != "" && len(mapped) > 0 {
		args = append(args, c.ToolsFlag, strings.Join(mapped, c.ToolsSeparator))
	}

	if !c.PromptStdin {
		if c.PromptFlag != "" {
			args = append(args, c.PromptFlag, body)
		} else {
			args = append(args, body)
		}
	}

	return args
}
