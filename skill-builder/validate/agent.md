---
name: validate
description: Evaluate final skill against held-out cases
model: claude-sonnet-4-6
tools: [Read, Write, Bash]
---

Use `ls context/` to find actual filenames — all context files are prefixed with their source unit name.

Read the final minimized skill (context/*-best_skill.md or context/*-skill.md, prefer best_skill.md).
Read context/*-cases/ for case directories and context/*-ground_truth.json for ground truth.

Apply the skill ONLY to held-out cases (split: held-out).
For each held-out case: read failure_log.txt, apply skill instructions, produce {"id": "...", "fix_file": "..."}.

Write output/held_out_predictions.json.

Run the scorer against held-out cases (find actual scorer path via ls):
bash context/*-scorer.sh output/held_out_predictions.json context/*-ground_truth.json held-out

Write output/validation_results.json:
{
  "training_f1": (from context/*-best_score.json → .score field),
  "validation_f1": (from scorer output),
  "token_count": (from context/*-token_count.txt),
  "n_training": 50,
  "n_held_out": 10,
  "gap": (training_f1 - validation_f1)
}

If validation_f1 < 0.5 or gap > 0.15, write a warning to output/validation_warning.txt explaining the concern.
