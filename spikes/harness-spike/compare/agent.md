---
model: claude-sonnet-4-6
tools: [Read, Write]
---

# Spike: Harness comparison

Read the three research outputs and produce a recommendation for which harness(es)
JUC should build adapters for first.

Inputs (staged into context/ by the runner):
- `context/pi-pi.md`
- `context/opencode-opencode.md`
- `context/goose-goose.md`

JUC's requirements for a harness adapter:

1. **Headless mode** — must be invocable as a subprocess that exits cleanly
2. **System prompt control** — must accept a custom system prompt per-invocation
3. **Model + provider** — must support OpenRouter (or generic OpenAI-compat endpoint)
4. **Tool execution** — must have working file read/write and bash tools
5. **Output capture** — stdout or file output that JUC can verify with checks

## Output

Write `output/recommendation.md` with:

### Comparison table

| Criterion | Pi | OpenCode | Goose |
|-----------|-----|----------|-------|
| Headless mode | | | |
| System prompt | | | |
| OpenRouter | | | |
| Tools | | | |
| Output capture | | | |
| Adapter effort | | | |

### Recommended build order

Which adapter to build first and why. Which is a blocker (e.g. no headless mode).

### Invocation templates

For each viable harness, the exact command `XxxAgent.Execute` would call, modeled
on JUC's current `CLIAgent`:

```
claude --agent <agentPath> --print <body> --dangerously-skip-permissions
```

### Open questions

Anything the spike couldn't resolve that needs a follow-up (e.g. needs local install
to test, undocumented flag, needs issue filed upstream).
