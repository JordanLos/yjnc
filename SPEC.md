# JUC Specification
## Version 2.0

---

### Abstract

JUC (Just Use Claude) is a minimum viable specification for Claude-native agentic orchestration. It defines a file-based structure for units of agent work, a directed acyclic graph model for composing those units, and a deterministic execution contract that bounds stochastic agent behavior.

JUC is not a framework. It specifies the minimum constraints required for correct, auditable, Claude-native agentic orchestration. Everything above that is developer expression.

JUC is a build system. The build steps are stochastic agents instead of deterministic scripts. Everything else — DAGs, artifacts, checks, retries, parallelism, hooks — is solved build infrastructure.

---

### Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

**Unit** — A directory containing a bounded piece of agent work: an agent definition (agent.md), optional human-provided input (context/), and recorded output (output/).

**Root** — The top-level directory of a JUC project. Contains graph.yaml, optional shared checks, and zero or more units.

**Runner** — The `juc` CLI. Reads graph.yaml, validates it against the schema, and orchestrates unit execution.

**Graph** — A directed acyclic graph (DAG) of units expressed in graph.yaml, where edges encode ordering and artifact flow.

**Check** — Any executable that verifies a unit's output deterministically. Checks exit 0 (pass) or non-zero (fail). Multiple checks may be composed per unit.

**Sampling** — Running a unit N times to collect multiple outputs for self-consistency evaluation.

**Consistency** — A strategy for evaluating agreement across samples: `majority`, `all`, `any`, or a custom script.

**Hook** — A shell command or script invoked at a defined lifecycle point in runner or unit execution.

**Unit Identity** — The canonical path of a unit directory relative to root, using `/` as separator, with no leading slash.

---

### Design Principles

JUC makes one architectural bet: Claude Code is the only dependency worth taking. Code-based orchestration frameworks bake prompts into control flow, creating maintenance burden and implementation lock-in as the underlying model platform evolves. Anthropic's own surface area — skills, agents, plugins, CLAUDE.md — is consistently text-file-first. JUC extends that pattern down to the unit of work.

**The top level is deterministic. Every leaf node is an agent.**

The runner owns the graph: ordering, state, retry, sampling, checkpoints, and artifact propagation. Agents own the work inside each unit. These layers do not cross.

**The only file you must write is agent.md.**

Everything else — checks, context, output, logs — is either optional or runner-managed. A unit with only agent.md is a valid, executable unit.

**JUC gives you five trust properties:**

1. **Pre-run legibility** — The entire plan is auditable before a single agent executes. agent.md and graph.yaml are human-readable by design.
2. **Execution guarantee** — The deterministic layer guarantees the graph runs as specified. Agents are always bounded by checks.
3. **Post-run auditability** — Logs record exactly what happened at each node.
4. **Resilience to platform change** — No framework to maintain. When Claude Code ships new capabilities, implementations use them immediately.
5. **Implementation freedom** — graph.yaml is the interface. The runner honors it; exports target it.

---

## 1. Spec Version

A JUC project declares its specification version in graph.yaml via the `juc` field.

```yaml
juc: "2.0"
```

The `juc` field MUST be present. The runner MUST reject graph.yaml files missing this field or declaring an unsupported version.

---

## 2. Unit of Work

A unit is a directory containing the following entries:

```
<unit>/
  agent.md      REQUIRED
  context/      OPTIONAL
  output/       RUNNER-MANAGED
```

### 2.1 agent.md

agent.md is a Claude Code subagent definition: YAML frontmatter followed by a Markdown system prompt body. Its schema is defined and owned by Claude Code. This specification does not constrain its fields.

```markdown
---
model: claude-sonnet-4-6
tools: [Read, Grep, Glob, Edit]
mcp: [playwright]
---

# Task

Implement the changes documented in `../research/output/findings.md`.

## Output

Write a PR description to `output/pr-body.md`.
```

agent.md is the complete specification of the unit. It contains the task, the tools, and the constraints. No other file is required.

### 2.2 context/

context/ is a directory of human-provided input files made available to the agent before it runs. Seed data, reference documents, and examples belong here.

context/ is for files that do not come from another unit. Artifact flow between units is declared via `depends` in graph.yaml and staged automatically by the runner.

context/ MUST NOT be modified during or after agent execution.

### 2.3 output/

output/ records the artifacts produced by unit execution. It is created and managed by the runner.

The runner MUST create output/ before invoking the agent. The agent writes its artifacts here. Downstream units receive declared output files as staged context.

---

## 3. Graph

### 3.1 graph.yaml

graph.yaml is the authoritative machine-readable representation of the project. It MUST be located in the root directory. It MUST conform to the JUC graph schema.

```yaml
juc: "2.0"

config:
  concurrency: 4
  logging: jsonl
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

Top-level keys are either reserved words (`juc`, `config`, `checks`) or unit identities. Any key not matching a reserved word is treated as a unit definition.

### 3.2 config

OPTIONAL. Runner-level configuration applied to the entire graph.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `concurrency` | integer | 4 | Maximum units executing concurrently |
| `logging` | `"jsonl"` \| `"none"` | `"jsonl"` | Log format written to `.juc/logs/` |
| `cache` | `"content-addressed"` \| `"mtime"` \| `"none"` | `"mtime"` | Cache strategy for skipping completed units |
| `secrets` | string[] | `[]` | Secret names required by this graph |
| `hooks` | object | — | Runner-level lifecycle hooks (see §5) |

### 3.3 checks

OPTIONAL. Named check groups, defined once and referenced by multiple units. Uses standard YAML anchor/alias syntax.

```yaml
checks:
  standard: &standard [lint, vitest]
  full: &full [lint, vitest, screenshot]
```

Named checks resolve to executable files in the root `checks/` directory. Path-prefixed values (`./verify.sh`) resolve relative to the unit.

### 3.4 Unit Definition

Each unit key maps to a unit configuration object.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `depends` | string[] | `[]` | Units that must pass before this unit runs |
| `verify` | string[] | `[]` | Checks to run after agent completes |
| `retries` | integer \| `"infinite"` | `0` | Retry attempts on check failure |
| `samples` | integer | `1` | Number of times to run the agent |
| `consistency` | string | `"any"` | Consistency strategy across samples |
| `timeout` | integer | — | Seconds before the agent is terminated |
| `hooks` | object | — | Unit-level lifecycle hooks (see §5) |

### 3.5 Graph Rules

The graph MUST be a directed acyclic graph. The runner MUST validate for cycles before execution begins and MUST halt with an error if any cycle is detected.

Units with no `depends` entries MAY begin execution immediately. Units with no path between them in the DAG MAY execute concurrently up to `config.concurrency`. Units connected by a `depends` edge MUST execute in dependency order.

Fan-in is supported: a unit MAY declare multiple `depends` entries. The runner MUST wait for all declared dependencies to pass before beginning the dependent unit.

---

## 4. Checks

### 4.1 checks/ Directory

The root MAY contain a `checks/` directory of shared executable check scripts. Named checks in unit `verify` fields resolve here.

```
<root>/
  checks/
    lint.sh
    vitest.sh
    screenshot.sh
```

Each check is an executable that exits 0 on success and non-zero on failure. Checks receive the unit's output directory as `$1`.

### 4.2 Composition

A unit's `verify` field is an ordered list. The runner MUST execute all listed checks in order. If any check exits non-zero, the unit fails. All checks MUST pass for the unit to pass.

### 4.3 Default Check

If a unit declares no `verify` checks, the runner MUST apply a default check: the unit's `output/` directory exists and contains at least one non-empty file.

---

## 5. Hooks

Hooks are shell commands or script paths executed at defined lifecycle points. They run as subprocesses of the runner.

### 5.1 Runner Hooks (config.hooks)

| Hook | Fires |
|------|-------|
| `before_run` | Once, before any unit begins |
| `after_run` | Once, after all units complete |
| `on_failure` | When any unit reaches state `failed` |

### 5.2 Unit Hooks (unit.hooks)

| Hook | Fires |
|------|-------|
| `before` | Before the agent runs for this unit |
| `after` | After all checks pass for this unit |
| `on_failure` | When this unit's checks fail |

Hooks receive the unit identity as `$1`. Runner hooks receive no arguments.

A non-zero hook exit MUST halt execution with an error, except `on_failure` hooks, which MUST NOT affect graph execution state.

---

## 6. Execution Model

### 6.1 Validation

Before execution begins, the runner MUST:

1. Parse graph.yaml and verify the `juc` field is present and the version is supported
2. Validate graph.yaml against the JUC schema
3. Validate the graph is a DAG — detect and reject cycles
4. Verify all declared unit directories exist on disk

### 6.2 Artifact Staging

Before executing a unit, the runner MUST copy the output files of each declared dependency into the unit's context/ directory. This MUST occur after all depends have passed.

Artifact flow is explicit. The runner MUST NOT move output between units unless declared via `depends`. There is no implicit data flow.

### 6.3 Execution Order

The runner MUST NOT begin a unit until all units in its `depends` list have passed. The runner MAY execute independent units concurrently up to `config.concurrency`.

### 6.4 Checks and Retry

After the agent completes, the runner executes the unit's checks in order. If all checks pass, the unit transitions to `passed`.

If any check fails and `retries` is greater than `0`, the runner re-executes the agent and re-runs all checks. This repeats until checks pass or retry attempts are exhausted.

If `retries` is `"infinite"`, the runner retries without limit until checks pass.

If retries are exhausted, the unit transitions to `failed` and graph execution halts.

### 6.5 Sampling

When `samples` is greater than `1`, the runner MUST execute the agent N times, storing each run's output in `output/sample-<n>/`. After all samples are collected, the runner evaluates consistency.

**Consistency strategies:**

| Strategy | Behavior |
|----------|----------|
| `any` | First passing sample wins |
| `majority` | More than half of samples must produce passing checks |
| `all` | All samples must produce passing checks |
| `<path>` | Custom script receives all sample output directories as arguments |

Samples with no path between them MAY execute concurrently.

### 6.6 Idempotency

A unit whose output is cached MUST NOT be re-executed unless invalidated.

When `cache` is `"content-addressed"`, a unit is cached by hashing its agent.md and all staged context files. When `cache` is `"mtime"`, a unit is skipped if its output directory is newer than its inputs. When `cache` is `"none"`, units always execute.

When a unit is invalidated, all transitively dependent units MUST also be invalidated.

### 6.7 State

The runner MUST track unit state and persist it to `.juc/state.json` after each transition.

Valid states: `pending`, `running`, `passed`, `failed`.

```
pending → running → passed
                 ↘ failed → [retry] → running
                          → [halt]
```

---

## 7. Logging

The runner MUST write structured logs to `.juc/logs/<unit>/run-<n>.jsonl`. Log files are runner-managed and invisible to unit authors.

Each line is a newline-delimited JSON object. Two event types are defined:

**tool_call:**
```json
{
  "type": "tool_call",
  "timestamp": "<ISO 8601>",
  "unit": "<unit-identity>",
  "tool": "<tool-name>",
  "input": {},
  "output": {}
}
```

**check_result:**
```json
{
  "type": "check_result",
  "timestamp": "<ISO 8601>",
  "unit": "<unit-identity>",
  "check": "<check-name>",
  "exit_code": 0,
  "stdout": "",
  "stderr": ""
}
```

`type`, `timestamp`, and `unit` are REQUIRED on every entry. All other fields are RECOMMENDED. Implementations MAY add additional event types. Conforming readers MUST ignore unknown fields.

---

## 8. Runner CLI

The runner is the `juc` CLI. It MUST be invocable from any directory containing a graph.yaml.

### 8.1 Commands

| Command | Description |
|---------|-------------|
| `juc run` | Execute the full graph |
| `juc run <unit>` | Execute a single unit and its dependencies |
| `juc status` | Print current state of all units |
| `juc logs <unit>` | Stream logs for a unit |
| `juc export --github-actions` | Generate a GitHub Actions workflow |

### 8.2 Secrets

Secrets declared in `config.secrets` are read from the environment or a `.env` file at root. The runner MUST error before execution if any declared secret is missing.

---

## 9. GitHub Actions Export

`juc export --github-actions` generates a `.github/workflows/juc.yml` from the current graph.

The exporter MUST:
- Map each unit to a GitHub Actions job
- Map `depends` to `needs`
- Inline agent.md content as the prompt input to `claude-code-action`
- Translate agent.md frontmatter fields to `claude_args`
- Inject `actions/upload-artifact` after each unit's checks pass
- Inject `actions/download-artifact` before each unit with declared dependencies
- Map `config.secrets` to `${{ secrets.* }}` references
- Map `samples` to a `strategy: matrix` on the job
- Map named checks to equivalent job steps

The generated workflow MUST NOT be committed automatically. The operator reviews and commits it.

---

## 10. Conformance

An implementation conforms to JUC 2.0 if it satisfies all MUST and MUST NOT requirements in this specification.

**A conforming runner MUST:**
- Parse and validate graph.yaml before execution, including version check, schema validation, and cycle detection
- Reject graphs containing cycles
- Respect `depends` ordering
- Stage dependency artifacts before dependent units execute
- Execute default check when no `verify` checks are declared
- Execute declared checks in order; fail unit if any check fails
- Retry up to `retries` times on check failure; retry without limit when `retries` is `"infinite"`
- Halt graph execution on unrecovered failure
- Skip units whose cache is valid
- Invalidate a unit and its transitive dependents when cache is invalidated
- Write state transitions to `.juc/state.json`
- Write logs to `.juc/logs/<unit>/run-<n>.jsonl`
- Error before execution if any declared secret is missing

**A conforming runner SHOULD:**
- Execute independent units concurrently
- Implement sampling and consistency evaluation
- Implement hooks

---

## 11. Extensions

This specification does not define an extension mechanism. Implementations MAY add graph.yaml fields, log event types, CLI commands, and behaviors not defined here, provided they do not conflict with any MUST or MUST NOT requirement.

Implementations MUST NOT require extensions for basic conformance.

Conforming runners MUST preserve unknown fields in graph.yaml and MUST ignore fields they do not recognize.

agent.md is owned entirely by Claude Code. This specification does not constrain its schema or behavior beyond its role as the agent definition for a unit.

---

## Appendix A: Reference Directory Structure

```
<root>/
  graph.yaml
  checks/
    lint.sh
    vitest.sh
    screenshot.sh
  research/
    agent.md
    context/
    output/
      findings.md
  implement/
    agent.md
    context/
    output/
      pr-body.md
  review/
    agent.md
    output/
  .juc/
    state.json
    logs/
      research/
        run-1.jsonl
      implement/
        run-1.jsonl
      review/
        run-1.jsonl
```

---

## Appendix B: graph.yaml Example

A four-unit pipeline with parallel research streams, shared checks, sampling, and hooks:

```yaml
juc: "2.0"

config:
  concurrency: 4
  logging: jsonl
  cache: content-addressed
  secrets:
    - ANTHROPIC_API_KEY
  hooks:
    before_run: ./scripts/setup.sh
    on_failure: ./scripts/notify.sh

checks:
  standard: &standard [lint, vitest]
  full: &full [lint, vitest, screenshot]

fetch:
  retries: 3

research:
  retries: 3

analyze:
  depends: [fetch, research]
  verify: *standard
  retries: infinite

implement:
  depends: [analyze]
  verify: *full
  retries: infinite
  hooks:
    before: ./scripts/branch.sh

review:
  depends: [implement]
  samples: 3
  consistency: majority
  verify: [screenshot]
```

`fetch` and `research` have no dependencies and run concurrently. `analyze` is a fan-in node. `implement` retries until all checks pass. `review` runs three samples and requires majority consistency.

---

## Appendix C: agent.md Frontmatter

agent.md frontmatter is defined and owned by Claude Code. Common fields:

```yaml
---
model: claude-sonnet-4-6
tools: [Read, Grep, Glob, Edit, Bash]
mcp: [playwright, github]
isolation: worktree
---
```

Refer to Claude Code documentation for the authoritative field reference. This specification does not constrain or extend the agent.md schema.
