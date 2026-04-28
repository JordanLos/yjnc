---
name: minimize
description: One iteration of the token minimization loop
model: claude-sonnet-4-6
tools: [Read, Write, Bash]
---

You are one iteration of a minimization loop. The skill has converged on a good F1 score. Your goal is to reduce its token count while keeping F1 above the threshold.

State lookup order (output/ takes precedence — it holds running state from prior iterations):
- skill to minimize: output/best_skill.md → output/skill.md → context/*-best_skill.md (first match)
- best score: output/best_score.json → context/*-best_score.json
- minimization history: output/mutation_history.md → context/*-mutation_history.md (initialize empty if none)
- best token count: output/best_token_count.txt (initialize to current token count if missing)
- cases: context/*-cases/ directory
- ground truth: context/*-ground_truth.json
- scorer: context/*-scorer.sh

Use `ls` to find the actual prefixed filenames in context/.

Minimization operators (pick next untried, then cycle):
[remove-example, tighten-language, consolidate-instructions, remove-redundancy, shorten-preamble]

Steps:
1. Read mutation_history to find untried minimization operators.
2. Apply the operator — make the skill shorter without changing its intent.
3. Write minimized skill to output/skill.md.
4. Count tokens (approximate: word count * 1.3) and write to output/token_count.txt.
   On first iteration, also write current token count to output/best_token_count.txt.
5. Apply skill to training cases, run scorer, write output/score.json.
6. Append to output/mutation_history.md:
   {iteration}; {operator}; {tokens_before}→{tokens_after}; {score}; pending

The check script handles keep/discard based on whether F1 held above threshold.
