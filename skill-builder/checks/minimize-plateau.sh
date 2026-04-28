#!/bin/bash
# Minimization plateau check
# $1 = unit output directory
OUTPUT=$1

SCORE=$(jq -r '.f1' "$OUTPUT/score.json" 2>/dev/null || echo "0")
BEST_SCORE=$(jq -r '.score' "$OUTPUT/best_score.json" 2>/dev/null || echo "0")
THRESHOLD=$(echo "$BEST_SCORE - 0.02" | bc -l)
TOKENS=$(cat "$OUTPUT/token_count.txt" 2>/dev/null || echo "9999")
BEST_TOKENS=$(cat "$OUTPUT/best_token_count.txt" 2>/dev/null || echo "9999")
NO_IMP=$(cat "$OUTPUT/no_improvement.txt" 2>/dev/null || echo "0")

# Did score hold above threshold?
HELD=$(echo "$SCORE >= $THRESHOLD" | bc -l)

if [ "$HELD" -eq 1 ] && [ "$TOKENS" -lt "$BEST_TOKENS" ]; then
  cp "$OUTPUT/skill.md" "$OUTPUT/best_skill.md"
  echo "$TOKENS" > "$OUTPUT/best_token_count.txt"
  echo "0" > "$OUTPUT/no_improvement.txt"
  echo "Minimized: $BEST_TOKENS → $TOKENS tokens (F1 held at $SCORE)" >&2
  exit 1
fi

NO_IMP=$((NO_IMP + 1))
echo "$NO_IMP" > "$OUTPUT/no_improvement.txt"

if [ "$NO_IMP" -ge 5 ]; then
  echo "Minimization plateau ($NO_IMP consecutive no-improvements)" >&2
  cp "$OUTPUT/best_skill.md" "$OUTPUT/skill.md" 2>/dev/null
  exit 0
fi

cp "$OUTPUT/best_skill.md" "$OUTPUT/skill.md" 2>/dev/null
echo "Minimization failed: score $SCORE vs threshold $THRESHOLD ($NO_IMP/5)" >&2
exit 1
