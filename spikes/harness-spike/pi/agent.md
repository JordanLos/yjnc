---
model: claude-sonnet-4-6
tools: [Bash, Write]
---

# Spike: Pi harness

Research Pi as a JUC harness adapter target. Pi is a terminal coding agent with
40+ provider support. Two repos — investigate both, prefer the more active fork:

- https://github.com/can1357/oh-my-pi (fork, binary: `omp`)
- https://github.com/badlogic/pi-mono (original)

Use `curl -s https://raw.githubusercontent.com/<owner>/<repo>/main/README.md` to
fetch docs. Also check `--help` output if the binary is installed (`which omp`).

Answer:

1. **Non-interactive invocation** — exact flag to run a prompt and exit with no REPL
   (equivalent of `claude --print`). What is the full invocation template?
2. **System prompt** — flag to pass a system prompt file or inline string
3. **Model + provider** — flag to set model ID and provider (especially OpenRouter)
4. **Tools** — built-in tools available (file read/write/edit, bash, search). How
   are they enabled or restricted per-invocation?
5. **Output** — where does the agent's output go? stdout? a file? structured JSON?
6. **Subagents** — does Pi support spawning subagents? If so, how?

## Output

Write `output/pi.md` with:

- One-line invocation template showing the command JUC's `PiAgent.Execute` would call
- Flag reference table (flag, purpose, example)
- How to configure OpenRouter as the provider
- Tool control mechanism
- Adapter feasibility: what maps cleanly to JUC's needs, what doesn't
