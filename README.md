# juc

**Just Use Context.**

JUC is a minimum viable specification for AI-native agentic orchestration. A directory shape, a YAML graph, a checks interface. No framework. No SDK. No language requirement.

> *The minimum viable constraint set for correct, auditable, AI-native agentic orchestration. Everything above that is developer expression.*

---

## The bet

Agent frameworks like LangChain and Mastra take a code-first approach: prompts baked into control flow, orchestration logic written in Python or TypeScript, framework versions to pin and maintain. Every time a new model or harness ships, you wait for the framework to catch up.

JUC follows the text-file-first grain of modern AI tooling all the way down. The spec is a directory shape — it can't go stale. graph.yaml and agent.md are plain text. The runner is an implementation detail.

The bet: **the orchestration layer should be harness-agnostic. When Claude ships new capabilities, you use them immediately. When a better harness ships, you point units at it. The spec doesn't change.**

---

## The architecture

```
The top level is deterministic. Every leaf node is an agent.
```

JUC is a build system where the build steps are stochastic agents instead of deterministic scripts. Everything else — DAGs, artifacts, checks, retries, parallelism, hooks — is solved build infrastructure.

The **runner** (`juc` CLI) owns the graph: ordering, state, retry, sampling, and artifact propagation. All deterministic.

**Agents** own the work inside each unit. Stochastic, bounded by checks.

**Harnesses** bridge the two: the runner dispatches each unit to the appropriate AI coding harness (Claude Code, Forge, Goose, Pi, OpenCode). Claude is the default.

These layers never cross.

---

## The structure

The only file you must write is **agent.md**:

```
<unit>/
  agent.md      system prompt + frontmatter (harness, model, tools)
  context/      optional human-provided seed files
  output/       runner-managed artifacts
```

The **root**:

```
<root>/
  graph.yaml    DAG, policy, checks, hooks
  checks/       shared check scripts (lint.sh, vitest.sh, ...)
  <unit-a>/
  <unit-b>/
  .juc/         work log — runner state, run logs, cache (commit this)
```

---

## graph.yaml

```yaml
juc: "2.0"

config:
  concurrency: 4
  harness: claude      # default for all units; override per-unit in agent.md
  secrets:
    - ANTHROPIC_API_KEY

checks:
  standard: &standard [lint, vitest]

research:
  retries: 3

implement:
  depends: [research]
  verify: *standard
  retries: infinite

review:
  depends: [implement]
  samples: 3
  consistency: majority
  verify: [screenshot, *standard]
```

---

## agent.md

Units can declare their harness, model tier, and tool allowlist in frontmatter:

```markdown
---
harness: forge
model: sonnet
tools: read, write, edit, bash
---

Your task is to implement the feature described in context/spec.md.
```

Omit the frontmatter to inherit the graph default (`config.harness`), or Claude if none is set. Model tiers (`haiku` / `sonnet` / `opus`) resolve to the current concrete model ID for each harness — your graph stays current across model releases without edits.

---

## Harnesses

The runner ships built-in support for five harnesses:

| Harness | Status | Notes |
|---|---|---|
| `claude` | stable | Claude Code — reference implementation |
| `forge` | experimental | ForgeCode — spec-native, adapter ships with CLI |
| `goose` | needs-adapter | Block's Goose — recipe model, adapter gaps remain |
| `pi` | needs-adapter | Pi/omp — close to spec-native |
| `opencode` | needs-research | OpenCode — headless story unconfirmed |

**Adapter protocol** — for harnesses without native flag support, the runner resolves an adapter binary:

1. `.juc/adapters/<name>` (project-local)
2. `~/.juc/adapters/<name>` (global)
3. `juc-adapter-<name>` on PATH

The adapter receives `JUC_UNIT`, `JUC_UNIT_DIR`, `JUC_AGENT_MD`, `JUC_ATTEMPT`, `JUC_ROOT` as env vars. It is responsible for translating agent.md, invoking the harness, and writing to `output/`. Exit 0 = success, non-zero = failure.

Override any built-in by placing a custom config at `.juc/harnesses/<name>.yaml`.

---

## Five reasons to trust it

**Before the run** — The entire plan is auditable in plain English before a single agent executes. agent.md and graph.yaml are human-readable by design.

**During the run** — The deterministic layer guarantees the graph runs as specified. Agents are always bounded by checks.

**After the run** — Logs record exactly what happened at each node. The output is always derivable from the current spec.

**Long-term** — No framework to maintain. When Claude Code ships new capabilities, you use them immediately. When a better harness ships, you point units at it. The spec doesn't change.

**Implementation freedom** — graph.yaml is the interface. The `juc` CLI honors it; any harness can target it.

---

## Why not LangChain or Mastra?

They bake prompts into control flow. You reason through code paths and natural language simultaneously. Prompts scatter across files. The framework versions lag behind model releases.

JUC separates concerns cleanly: the runner does deterministic things (checks, graph traversal), text does reasoning things (agent.md, context/). Never mixed. The harness is a runtime choice — not baked into your graph.

| | JUC | LangChain / Mastra |
|---|---|---|
| Prompt location | agent.md — a text file | baked into code |
| Audit trail | structural, free | build it yourself |
| Verification | first-class checks | not a concept |
| New model ships | use it immediately | wait for framework update |
| Harness | swappable at runtime | locked to framework |
| Language requirement | none | Python or TypeScript |
| Debugging | read the files | framework stacktraces |

---

## Spec

The full specification is in [SPEC.md](SPEC.md). It uses RFC 2119 conformance language and is precise enough to implement independently in any language.

Schema for graph.yaml: [schema/graph.schema.json](schema/graph.schema.json)

---

## Examples

- [examples/pr-build](examples/pr-build) — research → implement pipeline for building a PR
- [examples/makefile-runner](examples/makefile-runner) — a minimal 1.0-era runner using Make
- [skill-builder](skill-builder) — autoresearch loop for building Claude Code skills

---

## License

The JUC specification is © Jordan Scales and released under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). You may use, implement, and adapt the spec freely, with attribution. Derivatives must carry attribution and use the same license. The canonical specification is maintained at [github.com/JordanLos/just-use-context](https://github.com/JordanLos/just-use-context).
