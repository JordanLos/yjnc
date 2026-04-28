#!/usr/bin/env bash
# Usage: scorer.sh predictions.json ground_truth.json [split]
# split: "training" | "held-out" | "all"  (default: all)
#
# predictions.json: list or {"cases": [...]} of {"id": "...", "fix_file": "..."|null}
# ground_truth.json: list of {"id": "...", "split": "...", "fix_file": "..."|null}
#
# Metric: exact-match micro-F1 over all GT cases in the requested split.
#   TP = prediction.fix_file == GT.fix_file  (both non-null, exact string match)
#   FP = prediction.fix_file is non-null and != GT.fix_file
#   FN = GT.fix_file is non-null and prediction is null or wrong
#   Precision = TP / (TP + FP),  Recall = TP / (TP + FN)
#   F1 = 2*P*R / (P+R),  or 0 if both are 0
#   Null-vs-null (both predict and GT are null) counts as a correct non-positive (TN).

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 predictions.json ground_truth.json [split]" >&2
  exit 1
fi

PREDICTIONS="$1"
GROUND_TRUTH="$2"
SPLIT="${3:-all}"

python3 - "$PREDICTIONS" "$GROUND_TRUTH" "$SPLIT" <<'PYEOF'
import json, sys

pred_file, gt_file, split = sys.argv[1], sys.argv[2], sys.argv[3]

with open(pred_file) as f:
    raw = json.load(f)

# Accept both list and {"cases": [...]}
if isinstance(raw, dict) and "cases" in raw:
    pred_list = raw["cases"]
elif isinstance(raw, list):
    pred_list = raw
else:
    print(f"error: {pred_file} must be a list or {{\"cases\": [...]}}", file=sys.stderr)
    sys.exit(1)

pred_map = {p["id"]: p.get("fix_file") for p in pred_list}

with open(gt_file) as f:
    gt_list = json.load(f)

if not isinstance(gt_list, list):
    print(f"error: {gt_file} must be a JSON array", file=sys.stderr)
    sys.exit(1)

tp = fp = fn = tn = 0
misses = []

for case in gt_list:
    if split != "all" and case.get("split") != split:
        continue
    cid = case["id"]
    gt_fix = case.get("fix_file")
    pred_fix = pred_map.get(cid)

    if gt_fix is None and pred_fix is None:
        tn += 1
    elif gt_fix is not None and pred_fix == gt_fix:
        tp += 1
    elif pred_fix is not None and pred_fix != gt_fix:
        fp += 1
        if gt_fix is not None:
            fn += 1
        misses.append({"id": cid, "predicted": pred_fix, "expected": gt_fix})
    else:
        # pred_fix is None, gt_fix is not None
        fn += 1
        misses.append({"id": cid, "predicted": pred_fix, "expected": gt_fix})

total = tp + fp + fn + tn
precision = tp / (tp + fp) if (tp + fp) > 0 else 0.0
recall    = tp / (tp + fn) if (tp + fn) > 0 else 0.0
f1        = 2 * precision * recall / (precision + recall) if (precision + recall) > 0 else 0.0

print(f"Split:     {split}")
print(f"Total:     {total}  (TP={tp} FP={fp} FN={fn} TN={tn})")
print(f"Precision: {precision:.4f}")
print(f"Recall:    {recall:.4f}")
print(f"F1:        {f1:.4f}")

if misses:
    print("\nMisses:")
    for m in misses:
        print(f"  {m['id']}: predicted={m['predicted']!r}  expected={m['expected']!r}")
PYEOF
