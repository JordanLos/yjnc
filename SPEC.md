# YJNC Specification
## Version 1.0

---

### Abstract

YJNC (You Just Need Claude) is a minimum viable specification for Claude-native agentic orchestration. It defines a file-based structure for units of agent work, a directed acyclic graph model for composing those units, and a deterministic execution contract that bounds stochastic agent behavior.

YJNC is not a framework. It specifies the minimum constraints required for correct, auditable, Claude-native agentic orchestration. Everything above that is developer expression.

---

### Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

**Unit** — A directory containing a bounded piece of agent work: a specification (todo.md), an agent definition (agent.md), a verifier (contract), pre-collected input (context/), and recorded output (output/).

**Root** — The top-level directory of a YJNC project. Contains the work graph, runner, and zero or more units.

**Runner** — Any executable that reads work-graph.json and orchestrates unit execution according to this specification.

**Work Graph** — A directed acyclic graph (DAG) of units, expressed in work-graph.json, where edges encode ordering and data flow constraints.

**Contract** — Any executable within a unit that verifies the unit's output deterministically. The contract is the boundary between the deterministic runner layer and the stochastic agent layer.

**Preflight** — An optional preparation phase in which context is collected and populated into unit context/ directories before execution begins.

**Checkpoint** — A pause point in graph execution requiring human intervention before the runner may proceed.

**Sampling** — Running a unit N times to collect multiple outputs for self-consistency evaluation.

**Unit Identity** — The canonical path of a unit directory relative to root, using `/` as separator, with no leading slash.

---

### Design Principles

YJNC makes one architectural bet: Claude Code is the only dependency worth taking. Code-based orchestration frameworks bake prompts into control flow, creating maintenance burden and implementation lock-in as the underlying model platform evolves. Anthropic's own surface area — skills, agents, plugins, managed teams, CLAUDE.md — is consistently text-file-first. YJNC extends that pattern down to the unit of work.

**The top level is deterministic. Every leaf node is an agent.**

The runner owns the graph: ordering, state, retry, sampling, checkpoints, and context propagation. Agents own the work inside each unit. These layers do not cross.

**YJNC gives you five trust properties:**

1. **Pre-run legibility** — The entire plan is auditable in plain English before a single agent executes. todo.md, agent.md, and context/ are human-readable by design.
2. **Execution guarantee** — The deterministic layer guarantees the graph runs as specified. Agents are always bounded by a contract.
3. **Post-run auditability** — Logs, patches, and contract results record exactly what happened at each node. The output is always derivable from the current spec.
4. **Resilience to platform change** — No framework to maintain. When Claude Code ships new capabilities, implementations use them immediately.
5. **Implementation freedom** — The runner is any executable, in any language. The spec defines what; you define how.

---

## 1. Spec Version

A YJNC project declares its specification version in work-graph.json via the `yjnc` field.

```json
{
  "yjnc": "1.0"
}
```

The `yjnc` field MUST be present in work-graph.json. Runners MUST reject work-graph.json files that are missing this field or declare an unsupported version.

---

## 2. Unit of Work

A unit is a directory containing the following entries:

```
<unit>/
  todo.md       REQUIRED
  agent.md      REQUIRED
  contract      REQUIRED
  context/      OPTIONAL
  output/       OPTIONAL
```

### 2.1 todo.md

todo.md is a Markdown file containing a checklist of work items for this unit, written in plain English. It MAY contain wikilinks to other unit todo.md files using `[[<unit-identity>/todo]]` syntax to express the work graph for human readers.

todo.md MUST be written before the unit executes. It is the human-readable specification of the unit's work.

Wikilinks are informational. The authoritative machine-readable graph is work-graph.json.

### 2.2 agent.md

agent.md is a Claude Code subagent definition file: YAML frontmatter followed by a Markdown system prompt body. Its schema is defined and owned by Claude Code. This specification does not constrain its fields beyond its role as the agent definition for a unit.

The system prompt SHOULD reference the unit's todo.md and context/ as its primary inputs.

### 2.3 contract

contract is an executable file. The runner MUST execute contract after the agent completes and MUST interpret the exit code as follows:

| Exit code | Meaning |
|-----------|---------|
| `0` | Pass — unit output is verified complete |
| `1` | Fail — unit output did not meet requirements |
| `2` | Checkpoint — execution pauses for human intervention |

Any other exit code MUST be treated as `1` (fail).

contract MAY be any executable: a shell script, a compiled binary, a test suite invocation, a lint check, a database query, a CLI call, or any combination thereof. The only interface this specification defines is the exit code.

stdout and stderr from contract SHOULD be captured and appended to the unit's log.

### 2.4 context/

context/ is a directory of input files made available to the agent before it runs. If preflight is implemented, the runner SHOULD populate context/ before invoking the agent.

context/ MUST NOT be modified during or after agent execution.

### 2.5 output/

output/ records the artifacts and events produced by unit execution. Its contents are written by the runner and by agent hooks during the run.

#### 2.5.1 output/patches/

When the agent runs in an isolated worktree (via `isolation: worktree` in agent.md), the runner SHOULD write the resulting diff as a standard git patch file to output/patches/.

Patch files record what changed. They are not applied automatically. The patch is the record.

#### 2.5.2 output/log.jsonl

Each line of output/log.jsonl is a newline-delimited JSON object recording one event from the agent's execution. Two event types are defined by this specification:

**tool_call** — records a tool the agent invoked:
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

**side_effect** — records a filesystem or external change:
```json
{
  "type": "side_effect",
  "timestamp": "<ISO 8601>",
  "unit": "<unit-identity>",
  "effect": "<effect-type>",
  "path": "<affected-path>"
}
```

`type`, `timestamp`, and `unit` are REQUIRED on every log entry. All other fields are RECOMMENDED.

Implementations MAY add additional event types and fields. Conforming readers MUST ignore unknown fields and event types.

log.jsonl SHOULD be written by a Claude Code PostToolUse hook.

---

## 3. Work Graph

### 3.1 work-graph.json

work-graph.json is the authoritative machine-readable representation of the project. It MUST be located in the root directory.

```json
{
  "yjnc": "1.0",
  "units": {
    "<unit-identity>": {
      "depends": ["<unit-identity>"],
      "context_from": ["<path>"]
    }
  },
  "policy": {
    "on_failure": "halt",
    "retry": 0,
    "sampling": 1
  },
  "state": {
    "<unit-identity>": "pending"
  }
}
```

#### 3.1.1 units

REQUIRED. An object mapping unit identities to their graph configuration.

**units.\<id\>.depends** — REQUIRED (MAY be an empty array). Unit identities that MUST reach state `passed` before this unit may execute.

**units.\<id\>.context_from** — OPTIONAL. Paths relative to root whose contents the runner MUST copy into this unit's context/ directory before the agent runs. Evaluated after all depends have passed.

#### 3.1.2 policy

OPTIONAL. Execution policy applied to all units.

**policy.on_failure** — `"halt"` (default) or `"retry"`. Behavior when a contract exits `1`.

**policy.retry** — Integer. Maximum retry attempts when on_failure is `"retry"`. Default: `0`.

**policy.sampling** — Integer. Number of times to run each unit. Default: `1`.

#### 3.1.3 state

REQUIRED. An object mapping unit identities to their current execution state. The runner MUST update this field as execution proceeds and MUST persist it to disk after each transition.

Valid states: `pending`, `running`, `passed`, `failed`, `checkpoint`.

### 3.2 Graph Rules

The work graph MUST be a directed acyclic graph. The runner MUST validate the graph for cycles before execution begins. If a cycle is detected, the runner MUST halt with an error before executing any unit.

Units with an empty `depends` array MAY begin execution immediately.

Units with no path between them in the DAG MAY execute concurrently. Units connected by a `depends` edge MUST execute in dependency order.

Fan-in is supported: a unit MAY declare multiple entries in `depends`. The runner MUST wait for all declared dependencies to reach state `passed` before beginning the dependent unit.

### 3.3 Unit Identity

A unit's identity is its directory path relative to root, using `/` as the separator, with no leading slash.

| Unit directory | Identity |
|----------------|----------|
| `<root>/fetch/` | `fetch` |
| `<root>/phase-1/implement/` | `phase-1/implement` |

### 3.4 Wikilink Resolution

Wikilinks in todo.md files resolve relative to root. `[[analyze/todo]]` resolves to `<root>/analyze/todo.md`.

Wikilinks are informational. Runners are not required to parse them.

---

## 4. Root

The root is the top-level directory of a YJNC project.

```
<root>/
  work-graph.json   REQUIRED
  runner            REQUIRED
  todo.md           RECOMMENDED
  context/          OPTIONAL
  output/           OPTIONAL
  <unit>/           ZERO OR MORE
```

The root MUST NOT contain an agent.md. The root is the deterministic layer. Agents exist only inside units.

### 4.1 runner

runner is any executable that reads work-graph.json and orchestrates execution according to this specification. Its implementation language, runtime, and internal architecture are not constrained by this specification.

A Makefile, shell script, Go binary, or any other executable MAY serve as the runner, provided it satisfies all MUST requirements in Section 5.

### 4.2 Shared Context

The root MAY contain a context/ directory. Its contents are available to the runner for any purpose, including populating unit context/ directories via context_from edges.

---

## 5. Execution Model

### 5.1 Validation

Before execution begins, the runner MUST:

1. Parse work-graph.json and verify the `yjnc` field is present and the version is supported
2. Validate the graph is a DAG — detect and reject cycles
3. Verify all declared unit directories exist on disk
4. Re-evaluate contracts for units in state `passed` to confirm they still pass (idempotency)

### 5.2 Preflight

Before executing any unit, the runner SHOULD walk the full graph, collect relevant context, and populate each unit's context/ directory.

Preflight is RECOMMENDED. Implementations that omit preflight still conform to this specification.

### 5.3 Execution Order

The runner MUST NOT begin a unit until all units in its `depends` list have reached state `passed`.

The runner MAY execute independent units concurrently.

### 5.4 Context Propagation

When a unit declares `context_from`, the runner MUST copy the contents of each declared path into the unit's context/ directory before invoking the agent. This MUST occur after all depends have passed.

Context propagation is explicit. The runner MUST NOT move output between units unless explicitly declared via `context_from`. There is no implicit data flow.

### 5.5 Idempotency

A unit in state `passed` MUST NOT be re-executed unless explicitly invalidated.

A unit is invalidated when its agent.md is modified. When a unit is invalidated, all transitively dependent units MUST also be invalidated. Invalidated units return to state `pending`.

### 5.6 State Transitions

```
pending → running → passed
                 ↘ failed → [retry] → running
                          → [halt]  → (graph halts)
                 ↘ checkpoint → [human approves] → re-evaluate contract
                              → [human aborts]   → failed
```

The runner MUST write each state transition to work-graph.json before proceeding.

### 5.7 Retry

When a contract exits `1` and policy.on_failure is `"retry"`, the runner MUST re-execute the unit's agent and contract up to policy.retry times. If the contract continues to exit `1` after all retries are exhausted, the unit transitions to state `failed` and graph execution halts.

When policy.on_failure is `"halt"`, a contract exit of `1` immediately transitions the unit to `failed` and halts graph execution.

### 5.8 Checkpoint

When a contract exits `2`, the runner MUST:

1. Transition the unit to state `checkpoint` in work-graph.json
2. Halt execution of all dependent units
3. Surface the checkpoint to the operator for review

The runner MUST NOT proceed past a checkpoint without an explicit resume signal. The mechanism for signaling resume is implementation-defined.

On resume, the runner MUST re-evaluate the unit's contract. If it exits `0`, the unit transitions to `passed` and execution continues. If it exits `1` or `2`, the appropriate behavior repeats.

### 5.9 Sampling

When policy.sampling is greater than `1`, the runner MUST execute the unit's agent N times, storing each run's output in output/samples/\<n\>/. The contract is evaluated once after all samples are collected. Self-consistency logic is implementation-defined.

---

## 6. Conformance

An implementation conforms to YJNC 1.0 if it satisfies all MUST and MUST NOT requirements in this specification.

**A conforming runner MUST:**
- Parse and validate work-graph.json before execution, including version check and cycle detection
- Reject graphs containing cycles
- Respect `depends` ordering
- Propagate `context_from` when declared, and only when declared
- Implement the contract exit code interface: `0` (pass), `1` (fail), `2` (checkpoint)
- Persist state transitions to work-graph.json after each transition
- Halt graph execution on unrecovered failure
- Skip units in state `passed` (idempotency)
- Invalidate a unit and its transitive dependents when agent.md is modified

**A conforming runner SHOULD:**
- Execute independent units concurrently
- Implement preflight context collection
- Implement retry (policy.retry)
- Implement checkpoint with resume (exit code `2`)
- Implement sampling (policy.sampling)
- Capture contract stdout/stderr in the unit log
- Write output/log.jsonl via PostToolUse hooks
- Write output/patches/ for worktree-isolated agents

---

## 7. Extensions

This specification does not define an extension mechanism. Implementations MAY add files, directories, work-graph.json fields, log event types, and behaviors not defined here, provided they do not conflict with any MUST or MUST NOT requirement.

Implementations MUST NOT require extensions for basic conformance.

Conforming runners MUST preserve unknown fields in work-graph.json and MUST ignore fields they do not recognize.

agent.md is owned entirely by Claude Code. This specification does not constrain its schema or behavior beyond its role as the agent definition for a unit.

---

## Appendix A: Reference Directory Structure

```
<root>/
  work-graph.json
  runner
  todo.md
  context/
  output/
  fetch/
    todo.md
    agent.md
    contract
    context/
    output/
      patches/
      log.jsonl
  analyze/
    todo.md
    agent.md
    contract
    context/
    output/
      patches/
      log.jsonl
  implement/
    todo.md
    agent.md
    contract
    context/
    output/
      patches/
      log.jsonl
  verify/
    todo.md
    agent.md
    contract
    context/
    output/
      patches/
      log.jsonl
```

---

## Appendix B: work-graph.json Example

A four-unit pipeline where fetch and a parallel research stream converge before implementation, with a human checkpoint before final verification:

```json
{
  "yjnc": "1.0",
  "units": {
    "fetch": {
      "depends": []
    },
    "research": {
      "depends": []
    },
    "analyze": {
      "depends": ["fetch", "research"],
      "context_from": ["fetch/output", "research/output"]
    },
    "implement": {
      "depends": ["analyze"],
      "context_from": ["analyze/output"]
    },
    "verify": {
      "depends": ["implement"]
    }
  },
  "policy": {
    "on_failure": "retry",
    "retry": 2,
    "sampling": 1
  },
  "state": {
    "fetch":     "passed",
    "research":  "passed",
    "analyze":   "passed",
    "implement": "checkpoint",
    "verify":    "pending"
  }
}
```

`fetch` and `research` have no dependencies and run concurrently. `analyze` is a fan-in node that waits for both. `implement` is in `checkpoint` state — a human is reviewing output before verify proceeds.

---

## Appendix C: Minimal Makefile Runner

A Makefile is a conforming minimal runner for projects that do not require retry, sampling, or checkpoint support.

```makefile
.PHONY: all fetch analyze implement verify

all: verify

verify: implement
	./verify/contract

implement: analyze
	claude --agent implement/agent.md
	./implement/contract

analyze: fetch
	claude --agent analyze/agent.md
	./analyze/contract

fetch:
	claude --agent fetch/agent.md
	./fetch/contract
```

Run with `make -j` to execute independent targets concurrently.

A full conforming runner implementing retry, checkpoint, sampling, and state persistence in work-graph.json requires additional tooling beyond Make's native capabilities.
