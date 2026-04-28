#!/bin/bash
# Autoresearch plateau check
# $1 = unit output directory
OUTPUT=$1

SCORE=$(jq -r '.f1' "$OUTPUT/score.json" 2>/dev/null || echo "0")
BEST=$(jq -r '.score' "$OUTPUT/best_score.json" 2>/dev/null || echo "0")
NO_IMP=$(cat "$OUTPUT/no_improvement.txt" 2>/dev/null || echo "0")
ITER=$(jq -r '.iteration' "$OUTPUT/best_score.json" 2>/dev/null || echo "0")

# Hard cap
if [ "$ITER" -ge 25 ]; then
  echo "Hard cap reached (25 iterations)" >&2
  cp "$OUTPUT/best_skill.md" "$OUTPUT/skill.md" 2>/dev/null
  exit 0
fi

# Compare scores (use bc for float comparison)
IMPROVED=$(echo "$SCORE > $BEST" | bc -l)

if [ "$IMPROVED" -eq 1 ]; then
  cp "$OUTPUT/skill.md" "$OUTPUT/best_skill.md"
  echo "{\"score\": $SCORE, \"iteration\": $((ITER + 1))}" > "$OUTPUT/best_score.json"
  echo "0" > "$OUTPUT/no_improvement.txt"
  echo "Improved: $BEST → $SCORE" >&2
  exit 1  # keep iterating
fi

NO_IMP=$((NO_IMP + 1))
echo "$NO_IMP" > "$OUTPUT/no_improvement.txt"

if [ "$NO_IMP" -ge 5 ]; then
  echo "Plateau detected ($NO_IMP consecutive no-improvements)" >&2
  cp "$OUTPUT/best_skill.md" "$OUTPUT/skill.md" 2>/dev/null
  exit 0  # done
fi

# Revert to best, retry with different operator
cp "$OUTPUT/best_skill.md" "$OUTPUT/skill.md" 2>/dev/null
echo "No improvement ($NO_IMP/5): $SCORE vs best $BEST" >&2
exit 1
