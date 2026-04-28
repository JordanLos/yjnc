---
name: seed-skill
description: Write the naive seed skill for fault localization
model: claude-sonnet-4-6
tools: [Write]
---

Write two files:

1. output/skill.md — a naive fault localization skill. Keep it short and direct. It should instruct an agent to: read a CI failure log, identify which source file (not test file) likely needs to change to fix the failure, and output structured JSON. This is a seed for optimization — correctness matters more than sophistication.

2. output/mutation_history.md — empty history file with header only:
```
iter; operator; score_before; score_after; outcome
```

3. output/best_score.json:
{"score": 0.0, "iteration": 0}

4. output/no_improvement.txt:
0
