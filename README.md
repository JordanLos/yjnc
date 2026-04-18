# juc

**Just Use Claude.**

JUC is a minimum viable specification for Claude-native agentic orchestration. A directory shape, a YAML graph, a checks interface. No framework. No SDK. No language requirement.

> *The minimum viable constraint set for correct, auditable, Claude-native agentic orchestration. Everything above that is developer expression.*

---

## The bet

Agent frameworks like LangChain and Mastra take a code-first approach: prompts baked into control flow, orchestration logic written in Python or TypeScript, framework versions to pin and maintain. Every time Anthropic ships something new, you wait for the framework to catch up.

Claude Code is eating that layer. Subagents, agent teams, managed agents, skills, plugins — Anthropic's own surface area is consistently text-file-first. JUC follows that grain all the way down to the unit of work.

The bet: **Claude Code is the platform. Everything else is middleware waiting to be obsoleted.**

JUC has no middleware. When Claude Code ships new capabilities, you use them. The spec is a directory shape — it can't go stale.

---

## The architecture

```
The top level is deterministic. Every leaf node is an agent.
```

JUC is a build system where the build steps are stochastic agents instead of deterministic scripts. Everything else — DAGs, artifacts, checks, retries, parallelism, hooks — is solved build infrastructure.

The **runner** (`juc` CLI) owns the graph: ordering, state, retry, sampling, and artifact propagation. All deterministic.

**Agents** own the work inside each unit. Stochastic, bounded by checks.

These layers never cross.

---

## The structure

The only file you must write is **agent.md**:

```
<unit>/
  agent.md      Claude Code subagent definition (frontmatter + prompt)
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
  .juc/         runner state and logs (invisible to authors)
```

---

## graph.yaml

```yaml
juc: "2.0"

config:
  concurrency: 4
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

## Five reasons to trust it

**Before the run** — The entire plan is auditable in plain English before a single agent executes. agent.md and graph.yaml are human-readable by design.

**During the run** — The deterministic layer guarantees the graph runs as specified. Agents are always bounded by checks.

**After the run** — Logs record exactly what happened at each node. The output is always derivable from the current spec.

**Long-term** — No framework to maintain. When Claude Code ships new capabilities, you use them immediately. No update cycle.

**Implementation freedom** — graph.yaml is the interface. The `juc` CLI honors it; exports target it.

---

## Why not LangChain or Mastra?

They bake prompts into control flow. You reason through code paths and natural language simultaneously. Prompts scatter across files. The framework versions lag behind Anthropic releases.

JUC separates concerns cleanly: the runner does deterministic things (checks, graph traversal), text does reasoning things (agent.md, context/). Never mixed.

| | JUC | LangChain / Mastra |
|---|---|---|
| Prompt location | agent.md — a text file | baked into code |
| Audit trail | structural, free | build it yourself |
| Verification | first-class checks | not a concept |
| New Claude feature ships | use it immediately | wait for framework update |
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

---

## License

The JUC specification is © Jordan Scales and released under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). You may use, implement, and adapt the spec freely, with attribution. Derivatives must carry attribution and use the same license. The canonical specification is maintained at [github.com/JordanLos/juc](https://github.com/JordanLos/juc).
