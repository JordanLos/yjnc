---
name: build-scorer
description: Write the deterministic F1 scorer script
model: claude-sonnet-4-6
tools: [Read, Write]
---

Read context/ground_truth.json to understand the data structure.

Write output/scorer.sh — a bash script that:
- Takes $1 = predictions.json path, $2 = ground_truth.json path, $3 = split (training|held-out|all)
- Filters ground_truth to the requested split
- For each case, compares prediction fix_file against ground_truth fix_file
- Scoring: exact path match = 1.0, filename match (different directory) = 0.5, no match = 0.0
- Computes precision (correct predictions / total predictions) and recall (correct predictions / total ground truth cases)
- F1 = 2 * precision * recall / (precision + recall)
- Writes result to stdout as JSON: {"f1": 0.72, "precision": 0.80, "recall": 0.65, "n": 50}
- Writes per-case breakdown to stderr

Make scorer.sh executable. Test it with an empty predictions.json to confirm it runs without errors.

Write output/predictions_schema.json documenting the expected format for skill predictions:
{"cases": [{"id": "eslint-001", "fix_file": "lib/config/config.js"}]}
