---
model: claude-sonnet-4-6
tools: [Bash, Write]
---

# Spike: OpenCode harness

Research OpenCode as a JUC harness adapter target. OpenCode is an open source
terminal coding agent from SST, provider-agnostic, with 75+ provider support.

Repo: https://github.com/sst/opencode

Use `curl -s https://raw.githubusercontent.com/sst/opencode/main/README.md` to
fetch docs. Also check `packages/opencode/` for CLI source if docs are sparse.

Answer:

1. **Non-interactive invocation** — is there a flag to pass a prompt and exit with
   no interactive TUI? (equivalent of `claude --print`). If not, is there an SDK
   or RPC mode that JUC could drive programmatically?
2. **System prompt** — how to pass a custom system prompt per-invocation
3. **Model + provider** — how to set model ID and provider (especially OpenRouter).
   Config file vs CLI flag?
4. **Tools** — built-in tools available (file read/write/edit, bash, search). Can
   they be restricted per-invocation?
5. **Output** — where does agent output go? Can it be captured cleanly?
6. **Architecture** — note the client/server design and whether it affects
   programmatic use from a Go subprocess

## Output

Write `output/opencode.md` with:

- One-line invocation template (or note if headless mode doesn't exist yet)
- Flag reference table (flag, purpose, example)
- How to configure OpenRouter as the provider
- Tool control mechanism
- Adapter feasibility: what maps cleanly to JUC's needs, what doesn't, and any
  blockers (e.g. TUI-only, no stable headless mode)
