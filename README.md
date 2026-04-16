# yjnc

**You Just Need Claude.**

YJNC is a minimum viable specification for Claude-native agentic orchestration. A directory shape, a JSON graph, a contract interface. No framework. No SDK. No language requirement.

> *The minimum viable constraint set for correct, auditable, Claude-native agentic orchestration. Everything above that is developer expression.*

---

## The bet

Agent frameworks like LangChain and Mastra take a code-first approach: prompts baked into control flow, orchestration logic written in Python or TypeScript, framework versions to pin and maintain. Every time Anthropic ships something new, you wait for the framework to catch up.

Claude Code is eating that layer. Subagents, agent teams, managed agents, skills, plugins — Anthropic's own surface area is consistently text-file-first. YJNC follows that grain all the way down to the unit of work.

The bet: **Claude Code is the platform. Everything else is middleware waiting to be obsoleted.**

YJNC has no middleware. When Claude Code ships new capabilities, you use them. The spec is a directory shape — it can't go stale.

---

## The architecture

```
The top level is deterministic. Every leaf node is an agent.
```

The **runner** owns the graph: ordering, state, retry, sampling, checkpoints, context propagation. All deterministic.

**Agents** own the work inside each unit. Stochastic, bounded by a contract.

These layers never cross.

---

## The structure

A **unit of work**:

```
<unit>/
  todo.md       checklist in plain English, [[wikilinks]] to other units
  agent.md      Claude Code subagent definition
  contract      any executable — exit 0 pass, exit 1 fail, exit 2 checkpoint
  context/      pre-collected inputs for the agent
  output/
    patches/    git patches from worktree-isolated runs
    log.jsonl   tool calls and side effects, one JSON object per line
```

The **root**:

```
<root>/
  work-graph.json   authoritative DAG, execution state, policy
  runner            any executable that reads work-graph.json
  todo.md           human-readable overview
  context/
  <unit-a>/
  <unit-b>/
```

No `agent.md` at root. The root is deterministic only.

---

## Five reasons to trust it

**Before the run** — The entire plan is auditable in plain English before a single agent executes. todo.md, agent.md, and context/ are human-readable by design.

**During the run** — The deterministic layer guarantees the graph runs as specified. Agents are always bounded by a contract.

**After the run** — Logs, patches, and contract results record exactly what happened at each node. The output is always derivable from the current spec.

**Long-term** — No framework to maintain. When Claude Code ships new capabilities, you use them immediately. No update cycle.

**Implementation freedom** — The runner is any executable, in any language. A Makefile is a valid runner. A Go binary is a valid runner. The spec defines what; you define how.

---

## Why not LangChain or Mastra?

They bake prompts into control flow. You reason through code paths and natural language simultaneously. Prompts scatter across files. The framework versions lag behind Anthropic releases.

YJNC separates concerns cleanly: code does deterministic things (contracts, runner), text does reasoning things (agent.md, todo.md, context/). Never mixed.

| | YJNC | LangChain / Mastra |
|---|---|---|
| Prompt location | agent.md — a text file | baked into code |
| Audit trail | structural, free | build it yourself |
| Contract / verification | first-class | not a concept |
| New Claude feature ships | use it immediately | wait for framework update |
| Language requirement | none | Python or TypeScript |
| Debugging | read the files | framework stacktraces |

---

## Spec

The full specification is in [SPEC.md](SPEC.md). It uses RFC 2119 conformance language and is precise enough to implement independently in any language.

---

## Examples

- [examples/makefile-runner](examples/makefile-runner) — a minimal conforming runner using Make, with a four-unit sample pipeline

---

## License

The YJNC specification is © Jordan Scales and released under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). You may use, implement, and adapt the spec freely, with attribution. Derivatives must carry attribution and use the same license. The canonical specification is maintained at [github.com/JordanLos/yjnc](https://github.com/JordanLos/yjnc).
