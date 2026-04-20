---
model: claude-sonnet-4-6
tools: [Read, Grep, Glob]
---

# Research

Investigate the codebase and document what needs to change to implement the feature or fix described in context/.

## Output

Write findings to `output/findings.md`. Include:
- Relevant files and functions
- What needs to change and why
- Edge cases or risks

## Constraints

Read-only. Do not modify any source files.
