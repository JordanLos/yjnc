---
model: claude-sonnet-4-6
tools: [Bash, Write]
---

# Spike: Goose harness

Research Goose as a JUC harness adapter target. Goose is an open source coding
agent from Block (Square), multi-provider, extensible via plugins.

Repo: https://github.com/block/goose

Use `curl -s https://raw.githubusercontent.com/block/goose/main/README.md` to
fetch docs. Also check `docs/` and CLI reference if available.

Answer:

1. **Non-interactive invocation** — exact flag to run a prompt and exit with no
   REPL (equivalent of `claude --print`). Full invocation template?
2. **System prompt** — flag to pass a custom system prompt file or inline string
3. **Model + provider** — how to set model ID and provider (especially OpenRouter).
   Config file vs CLI flag?
4. **Tools / extensions** — built-in tools available (file read/write/edit, bash,
   search). Are tools configured globally or can they be set per-invocation?
5. **Output** — where does agent output go? Stdout? Structured?
6. **Recipe / profile system** — Goose has a recipe/profile concept. Does this map
   to JUC's per-unit agent definition pattern?

## Output

Write `output/goose.md` with:

- One-line invocation template showing the command JUC's `GooseAgent.Execute` would call
- Flag reference table (flag, purpose, example)
- How to configure OpenRouter as the provider
- Tool control mechanism
- Adapter feasibility: what maps cleanly to JUC's needs, what doesn't
