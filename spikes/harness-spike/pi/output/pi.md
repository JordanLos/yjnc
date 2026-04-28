# Pi / omp Harness Adapter Spike

**Repo evaluated:** `can1357/oh-my-pi` (fork, binary: `omp`) — significantly more active than `badlogic/pi-mono`, with 40+ providers, LSP, subagents, RPC mode, and an SDK. `omp` is the right target.

---

## Invocation Template

```bash
omp --print --no-session --mode json \
    --model openrouter/<model-id> \
    --system-prompt "<text-or-path>" \
    --tools read,write,edit,bash,grep,find \
    "<user-prompt>"
```

For plain-text output (simpler parsing):

```bash
omp --print --no-session \
    --model openrouter/<model-id> \
    --system-prompt "<text-or-path>" \
    "<user-prompt>"
```

---

## Flag Reference

| Flag | Purpose | Example |
|------|---------|---------|
| `--print` / `-p` | Non-interactive: run prompt and exit (equivalent to `claude --print`) | `omp -p "refactor foo"` |
| `--no-session` | Ephemeral — don't persist a session file | `omp --no-session -p "..."` |
| `--mode <mode>` | Output format: `text` (default), `json` (structured events), `rpc` (stdin/stdout JSON protocol) | `--mode json` |
| `--model <id>` | Model ID, supports provider prefix and fuzzy match | `--model openrouter/anthropic/claude-sonnet-4-5` |
| `--provider <name>` | Provider hint (legacy; prefer `--model`) | `--provider openrouter` |
| `--api-key <key>` | API key override (overrides env and stored credentials) | `--api-key $OPENROUTER_API_KEY` |
| `--system-prompt <text\|file>` | Replace default system prompt (accepts inline string or file path) | `--system-prompt ./AGENTS.md` |
| `--append-system-prompt <text\|file>` | Append to default system prompt without replacing | `--append-system-prompt "Always reply in JSON"` |
| `--tools <list>` | Restrict to comma-separated built-in tool names | `--tools read,grep,find` |
| `--no-tools` | Disable all built-in tools | `--no-tools` |
| `--no-lsp` | Disable LSP integration (reduces startup overhead) | `--no-lsp` |
| `--no-skills` | Disable skills discovery | `--no-skills` |
| `--no-rules` | Disable rules discovery | `--no-rules` |
| `--no-extensions` | Disable extension discovery | `--no-extensions` |
| `--thinking <level>` | Reasoning budget: `off`, `minimal`, `low`, `medium`, `high`, `xhigh` | `--thinking low` |
| `--session-dir <dir>` | Override session storage directory | `--session-dir /tmp/pi-sessions` |
| `--hook <path>` | Load a TypeScript hook file (repeatable) | `--hook ./juc-hook.ts` |

---

## OpenRouter Configuration

**Env var (recommended):**

```bash
export OPENROUTER_API_KEY=sk-or-...
```

omp auto-detects the key and makes all OpenRouter models available. No other config needed.

**Model selection:** Use `openrouter/<upstream-model-id>` as the `--model` value:

```bash
omp --model openrouter/anthropic/claude-sonnet-4-5 -p "..."
omp --model openrouter/meta-llama/llama-3-70b-instruct -p "..."
```

**Persist as default** via `~/.omp/agent/config.yml`:

```yaml
modelRoles:
  default: openrouter/anthropic/claude-sonnet-4-5
```

---

## Tool Control

omp ships 20 built-in tools. Three control mechanisms:

### Allow-list (`--tools`)

Pass a comma-separated list of the exact tool names to enable. All others are disabled:

```bash
omp --tools read,write,edit,bash,grep,find -p "..."
```

### Deny-all (`--no-tools`)

Disables every built-in tool. Useful for pure reasoning tasks or when tools are injected via MCP:

```bash
omp --no-tools -p "..."
```

### Full tool list

| Tool | What it does |
|------|-------------|
| `read` | Read files/directories |
| `write` | Create or overwrite files |
| `edit` | In-place file edits with hashline anchors |
| `bash` | Execute shell commands |
| `grep` | Search file content |
| `find` | Find files by glob |
| `ast_grep` | AST-aware code search |
| `ast_edit` | AST-aware code rewrites |
| `lsp` | Language server operations (11 actions) |
| `fetch` | Fetch and extract URL content |
| `web_search` | Multi-provider web search |
| `task` | Spawn subagents (parallel execution) |
| `poll` | Block on async background jobs |
| `python` | Execute Python in IPython kernel |
| `calc` | Deterministic calculator |
| `ssh` | Execute on remote SSH hosts |
| `browser` | Puppeteer browser automation |
| `notebook` | Edit Jupyter notebooks |
| `todo_write` | Phased task tracking |
| `generate_image` | Image generation via Gemini/OpenRouter |

---

## Output

- **Default (`--mode text`):** Agent output streams to **stdout** as plain text. Tool call results and thinking blocks are rendered in the terminal but are not emitted to stdout in print mode — only the final assistant text.
- **`--mode json`:** Structured event stream on stdout; each line is a JSON object (message_update, tool_call, etc.). Suitable for programmatic parsing.
- **`--mode rpc`:** Bidirectional JSON protocol on stdin/stdout. Send `{"type":"prompt","message":"..."}` commands; receive event objects. Best for long-lived adapter processes.
- No file-based output by default; redirect stdout as needed.

---

## Subagents

Yes — the `task` tool is a first-class subagent system:

- **6 bundled agents:** `explore`, `plan`, `designer`, `reviewer`, `task`, `quick_task`
- Agents run in parallel; results stream in real time
- Isolation backends: `none`, `worktree`, `fuse-overlay`, `fuse-projfs`
- Async background jobs with configurable concurrency (up to 100)
- Custom agents at `.omp/agents/` or `~/.omp/agent/agents/`
- Access full subagent output via `agent://<id>` resources

In non-interactive (`--print`) mode, the `task` tool still works — subagents run headlessly and their results surface in the parent's output.

---

## Adapter Feasibility

### Maps cleanly to JUC's needs

| JUC need | Pi/omp mapping |
|----------|---------------|
| Non-interactive execution | `--print` / `-p` flag — exact equivalent of `claude --print` |
| System prompt injection | `--system-prompt <text\|file>` replaces; `--append-system-prompt` appends |
| Provider/model selection | `--model openrouter/<id>` + `OPENROUTER_API_KEY` |
| Tool restriction | `--tools <list>` or `--no-tools` — fine-grained per-invocation control |
| Structured output | `--mode json` emits a parseable event stream |
| Subagents | Built-in `task` tool with 6 bundled agents |
| Hooks/observability | TypeScript hooks (`--hook <path>`) subscribing to `tool_call`, `message_update`, etc. |
| AGENTS.md / CLAUDE.md | Auto-discovered from project dirs — zero config for JUC-based projects |

### Gaps / friction points

| Issue | Detail |
|-------|--------|
| `omp` not pre-installed | Not on PATH by default; JUC adapter must document `bun install -g @oh-my-pi/pi-coding-agent` or use the installer script |
| JSON output format undocumented | `--mode json` event schema is not publicly documented; will need to reverse-engineer from source or live output |
| No `--cwd` flag | omp uses the shell's cwd; adapter must `cd` or spawn with correct `cwd` in process options |
| Interactive-only tools | `ask` tool requires a TTY; disable it or it may block in `--print` mode |
| Binary name collision | `omp` may conflict with other tools; worth aliasing or using full npm-bin path |

### Verdict

**Feasible.** The `--print --no-session --mode json --model --system-prompt --tools` combination maps directly to what `PiAgent.Execute` needs. The main implementation work is parsing the `--mode json` event stream to extract final text and tool outputs. RPC mode (`--mode rpc`) is an even better long-term fit for a persistent adapter process — avoids cold-start overhead and gives a clean request/response protocol.
