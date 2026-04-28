---
name: package
description: Bundle the skill with its benchmark evidence for distribution
model: claude-sonnet-4-6
tools: [Read, Write, Bash]
---

Use `ls context/` to find actual filenames — all context files are prefixed with their source unit name.

Read context/*-best_skill.md (or *-skill.md), context/*-validation_results.json, context/*-ground_truth.json, context/*-cases/.

Assemble the distributable skill package in output/:

output/skill.md — copy of best_skill.md

output/benchmark/
  ground_truth.json — the full ground truth (training + held-out)
  scorer.sh — the deterministic scorer
  scores.json — {training_f1, validation_f1, token_count, produced_at}

output/benchmark/cases/ — copy all case directories from context/cases/

output/README.md — document:
- What the skill does (fault localization: given a CI failure log, predict fix_file)
- Benchmark: what dataset, what metric, what scores
- How to run the scorer against a new skill: bash benchmark/scorer.sh predictions.json benchmark/ground_truth.json all
- Training F1 and validation F1 with token count

The README should make it possible for anyone to reproduce the benchmark score or compare a different skill against the same benchmark.
