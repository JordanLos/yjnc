#!/usr/bin/env python3
"""Apply skill to training cases and generate predictions.json"""
import json, subprocess, sys, re
from pathlib import Path

BASE = Path("/Users/jordan/code/juc/skill-builder/minimize")
CASES_DIR = BASE / "context/download-cases-cases"
GT_PATH = BASE / "context/structure-ground-truth-ground_truth.json"
SKILL_PATH = BASE / "output/skill.md"
PREDICTIONS_PATH = BASE / "output/predictions.json"

skill = SKILL_PATH.read_text().strip()
with open(GT_PATH) as f:
    gt = json.load(f)

training_ids = [c["id"] for c in gt if c["split"] == "training"]

results = []
for case_id in training_ids:
    log_path = CASES_DIR / case_id / "failure_log.txt"
    if not log_path.exists():
        print(f"  SKIP {case_id}: no failure_log.txt", file=sys.stderr)
        continue
    log_text = log_path.read_text()
    # Use last 200 lines to avoid huge logs
    lines = log_text.splitlines()[-200:]
    log_excerpt = "\n".join(lines)

    proc = subprocess.run(
        ["claude", "--print", "--system-prompt", skill,
         "--tools", "", "--model", "claude-haiku-4-5-20251001",
         "--no-session-persistence", log_excerpt],
        capture_output=True, text=True, timeout=30
    )
    output = proc.stdout.strip()

    # Extract JSON from output
    fix_file = None
    m = re.search(r'\{"file":\s*"([^"]+)"\}', output)
    if m:
        fix_file = m.group(1)
    else:
        # Try null
        m2 = re.search(r'\{"file":\s*null\}', output)
        if m2:
            fix_file = None
        else:
            # Try bare path extraction
            m3 = re.search(r'"([^"]*(?:lib|src)/[^"]+)"', output)
            if m3:
                fix_file = m3.group(1)

    print(f"  {case_id}: {fix_file}", file=sys.stderr)
    results.append({"id": case_id, "fix_file": fix_file})

predictions = {"cases": results}
with open(PREDICTIONS_PATH, "w") as f:
    json.dump(predictions, f, indent=2)
print(f"Wrote {len(results)} predictions to {PREDICTIONS_PATH}", file=sys.stderr)
