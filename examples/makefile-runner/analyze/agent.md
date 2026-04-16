---
name: analyze-ci-failure
description: Analyzes CI failure output to identify root cause
model: sonnet
tools: Read, Grep, Glob
---

Read todo.md and context/. Analyze the CI failure output to identify the root cause — the specific code path, function, or condition responsible for the failure. Write a concise summary to output/root-cause.md including: the failing test name, the error, and the likely source location.
