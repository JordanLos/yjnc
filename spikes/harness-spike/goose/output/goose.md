# Goose Harness Adapter Spike

Repo: https://github.com/aaif-goose/goose (moved from `block/goose` to AAIF/Linux Foundation)

---

## One-line invocation template

```
OPENROUTER_API_KEY=<key> goose run \
  --no-session \
  --no-profile \
  --with-builtin developer \
  --provider openrouter \
  --model anthropic/claude-sonnet-4 \
  --quiet \
  --text "<prompt>"
```

Provider/model can alternately be set via env (`GOOSE_PROVIDER`, `GOOSE_MODEL`) rather than CLI flags — either works. `--no-session` prevents creating session state files during automated runs.

---

## Flag reference

| Flag | Purpose | Example |
|---|---|---|
| `run` | Execute headless (non-interactive) | `goose run ...` |
| `--text / -t TEXT` | Inline prompt string | `--text "write a hello world"` |
| `--instructions / -i FILE` | Read prompt from file; `-i -` reads stdin | `--instructions prompt.txt` |
| `--recipe PATH` | Run a recipe YAML (conflicts with `--text`/`--instructions`) | `--recipe ./review.yaml` |
| `--system TEXT` | Append additional system prompt (incompatible with `--recipe`) | `--system "Reply in Spanish"` |
| `--params KEY=VALUE` | Key-value params for recipe templates (repeatable) | `--params branch=main` |
| `--provider NAME` | Override provider (e.g. `openrouter`, `anthropic`) | `--provider openrouter` |
| `--model NAME` | Override model ID | `--model anthropic/claude-sonnet-4` |
| `--with-builtin NAME` | Add a named built-in extension (comma-separated or repeatable) | `--with-builtin developer` |
| `--with-extension CMD` | Add a stdio MCP extension (repeatable) | `--with-extension "npx @modelcontextprotocol/server-git"` |
| `--with-streamable-http-extension URL` | Add an HTTP MCP extension | `--with-streamable-http-extension http://localhost:8080/mcp` |
| `--no-profile` | Skip loading configured profile extensions | `--no-profile` |
| `--no-session` | Run without creating a session file | `--no-session` |
| `--quiet / -q` | Suppress non-response output (only model response to stdout) | `--quiet` |
| `--output-format FORMAT` | `text` (default), `json`, `stream-json` | `--output-format json` |
| `--max-turns N` | Limit agent turns (default: 1000) | `--max-turns 50` |
| `--max-tool-repetitions N` | Limit consecutive identical tool calls | `--max-tool-repetitions 5` |
| `-s / --interactive` | Stay in REPL after processing initial input | `--interactive` |

---

## Configuring OpenRouter

**Environment variables (recommended for CI/automated use):**
```sh
export GOOSE_PROVIDER=openrouter
export GOOSE_MODEL=anthropic/claude-sonnet-4   # or any openrouter model slug
export OPENROUTER_API_KEY=sk-or-...
```

**Config file** (`~/.config/goose/config.yaml`):
```yaml
GOOSE_PROVIDER: openrouter
GOOSE_MODEL: anthropic/claude-sonnet-4
```
The API key is stored separately in the system keyring (via `goose configure`) or falls back to `~/.config/goose/secrets.yaml`. In non-interactive contexts, set `OPENROUTER_API_KEY` as an env var — it takes precedence over the keyring.

**CLI flags (per-invocation override):**
```sh
goose run --provider openrouter --model anthropic/claude-sonnet-4 ...
```
CLI flags override both env vars and config file.

**Optional**: Set `OPENROUTER_HOST` to override the base URL (default: `https://openrouter.ai`).

**Known OpenRouter model slugs:** `anthropic/claude-sonnet-4`, `anthropic/claude-opus-4`, `google/gemini-2.5-pro`, `google/gemini-2.5-flash`, `deepseek/deepseek-r1-0528`

---

## Tool control mechanism

Tools come from **extensions**. Extensions are either built-in, stdio MCP processes, or HTTP MCP servers.

### Built-in `developer` extension (the coding toolkit)

The `developer` builtin is the core coding extension. Add it with `--with-builtin developer`.

Tools it provides:
| Tool | What it does |
|---|---|
| `shell` | Execute shell commands (bash/zsh/sh via `$GOOSE_SHELL`) |
| `read` | Read file contents, with optional line/offset range |
| `write` | Write/overwrite a file |
| `edit` | Patch a file with before/after string replacement |
| `tree` | Show directory tree |

Other built-ins (add with `--with-builtin NAME`):
- `computercontroller` — screenshots, PDF/DOCX/XLSX reading, screen control
- `memory` — persistent memory across sessions
- `autovisualiser` — chart/diagram rendering
- `tutorial` — interactive tutorials

### Extension loading precedence

1. Profile extensions (from `~/.config/goose/config.yaml`) — loaded unless `--no-profile`
2. CLI-specified extensions (`--with-builtin`, `--with-extension`, `--with-streamable-http-extension`)
3. Recipe-defined extensions (when `--recipe` is used)

**For JUC's `GooseAgent.Execute`**: use `--no-profile --with-builtin developer` to get a clean, deterministic tool set — no user profile bleeds into the unit run.

---

## Output behavior

- Default: mixed text to **stdout** (agent reasoning + tool output interleaved, formatted for terminal)
- `--quiet`: only the final model response goes to stdout; tool calls and status suppressed
- `--output-format json`: single JSON blob on stdout at completion
- `--output-format stream-json`: newline-delimited JSON events streamed to stdout (useful for streaming output capture)
- Exit code is 0 on success, non-zero on error

For JUC, `--quiet --output-format text` gives the cleanest capture of just the agent's final response. `stream-json` is the right choice if JUC wants to stream turn-by-turn output.

---

## Recipe / profile system

A **recipe** is a YAML file that bundles a prompt template, system instructions, required parameters, and an extension list into a single reusable agent definition.

```yaml
version: "1.0.0"
title: PR Code Review
description: Review a GitHub pull request.

parameters:
  - key: branch
    input_type: string
    requirement: required
    description: Branch to review

extensions:
  - type: builtin
    name: developer
  - type: stdio
    name: my_tool
    cmd: npx
    args: ["@org/mcp-server"]

prompt: |
  Review the changes on branch {{ branch }}.
  ...
```

Invoked with:
```sh
goose run --recipe ./review.yaml --params branch=feature-x --no-session --quiet
```

**Mapping to JUC's per-unit agent definition:**

| JUC concept | Goose equivalent |
|---|---|
| Unit prompt / instructions | `prompt` + `instructions` fields in recipe |
| Unit system prompt | `--system TEXT` flag, or recipe-level instructions |
| Extension/tool set | Recipe `extensions` list or `--with-builtin` flags |
| Parametrized invocation | `--params key=value` matching `{{ key }}` in recipe template |
| Agent identity / persona | Recipe `title` + `description` |

Recipes map cleanly to JUC units when the unit has a stable prompt template and a fixed tool set. The `--text`/`--system` flags are the lighter-weight path for dynamic prompts.

---

## Adapter feasibility

### Maps cleanly

- **Headless execution**: `goose run --text` is a direct equivalent of `claude --print`. No REPL, exits on completion.
- **Provider + model selection**: both CLI flags and env vars give full per-invocation control; OpenRouter is a first-class provider.
- **Tool set control**: `--no-profile --with-builtin developer` gives a clean, reproducible tool set identical to Claude Code's core (shell, read, write, edit).
- **System prompt injection**: `--system` appends to the default system prompt at invocation time.
- **Output capture**: `--quiet` + stdout gives clean output; `stream-json` gives structured streaming.
- **Structured agent definitions**: recipes map to JUC unit configs almost 1:1.

### Friction points

- **No `--print` short form**: requires the full `run --text` subcommand; slightly more verbose than `claude --print`.
- **Session files by default**: must pass `--no-session` to avoid writing state to disk; easy to forget.
- **Profile bleed**: user's configured extensions load unless `--no-profile` is passed; determinism requires explicit suppression.
- **System prompt additive only**: `--system` appends to Goose's default developer system prompt, not replaces it. If JUC needs full system prompt control, a recipe with a custom `instructions` field is required.
- **Output format**: default `text` format includes ANSI/terminal formatting; `--quiet` strips most of it but some decoration may remain. `stream-json` is cleaner for programmatic consumption but requires JSON parsing.
- **Keyring for secrets**: default API key storage uses the OS keyring, which doesn't work headlessly. Env var override (`OPENROUTER_API_KEY`) works around this cleanly.
- **No stdin prompt by default**: prompt must be passed as `--text STRING` (shell quoting for long prompts) or via `--instructions -` (stdin). The stdin path is the cleanest for long prompts.
