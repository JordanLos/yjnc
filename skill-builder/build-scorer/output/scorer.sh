#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
    echo "Usage: $0 <predictions.json> <ground_truth.json> <split: training|held-out|all>" >&2
    exit 1
fi

if [[ "$3" != "training" && "$3" != "held-out" && "$3" != "all" ]]; then
    echo "Error: split must be one of: training, held-out, all" >&2
    exit 1
fi

python3 - "$1" "$2" "$3" <<'PYEOF'
import json, sys, os

predictions_path, ground_truth_path, split = sys.argv[1], sys.argv[2], sys.argv[3]

with open(predictions_path) as f:
    predictions_data = json.load(f)
with open(ground_truth_path) as f:
    gt_data = json.load(f)

gt_cases = gt_data if split == "all" else [c for c in gt_data if c["split"] == split]
gt_by_id = {c["id"]: c for c in gt_cases}
pred_cases = predictions_data.get("cases", [])
pred_by_id = {c["id"]: c for c in pred_cases}

def score_match(pred_fix, gt_fix):
    if pred_fix is None or gt_fix is None:
        return 0.0
    if pred_fix == gt_fix:
        return 1.0
    if os.path.basename(pred_fix) == os.path.basename(gt_fix):
        return 0.5
    return 0.0

# Per-case breakdown to stderr
print("Per-case breakdown:", file=sys.stderr)
for gt in gt_cases:
    gid = gt["id"]
    gt_fix = gt.get("fix_file")
    if gid in pred_by_id:
        pred_fix = pred_by_id[gid].get("fix_file")
        s = score_match(pred_fix, gt_fix)
        tag = "exact" if s == 1.0 else ("partial" if s == 0.5 else "miss")
    else:
        pred_fix = None
        s = 0.0
        tag = "missing"
    print(f"  [{tag:7}] {gid}: pred={pred_fix!r}  gt={gt_fix!r}  score={s}", file=sys.stderr)

for pred in pred_cases:
    pid = pred["id"]
    if pid not in gt_by_id:
        print(f"  [out-split] {pid}: pred={pred.get('fix_file')!r} (not in split, score=0)", file=sys.stderr)

# Precision: sum of scores for predictions / total predictions
pred_score_sum = sum(
    score_match(p.get("fix_file"), gt_by_id[p["id"]].get("fix_file")) if p["id"] in gt_by_id else 0.0
    for p in pred_cases
)
n_pred = len(pred_cases)
precision = pred_score_sum / n_pred if n_pred > 0 else 0.0

# Recall: sum of scores for ground truth cases / total ground truth cases
recall_score_sum = sum(
    score_match(pred_by_id[g["id"]].get("fix_file"), g.get("fix_file")) if g["id"] in pred_by_id else 0.0
    for g in gt_cases
)
n = len(gt_cases)
recall = recall_score_sum / n if n > 0 else 0.0

f1 = 2 * precision * recall / (precision + recall) if (precision + recall) > 0 else 0.0

print(json.dumps({"f1": round(f1, 4), "precision": round(precision, 4), "recall": round(recall, 4), "n": n}))
PYEOF
