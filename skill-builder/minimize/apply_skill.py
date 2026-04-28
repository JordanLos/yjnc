#!/usr/bin/env python3
"""Apply skill to training cases via claude CLI, produce predictions.json."""
import json, os, re, subprocess, sys

CASES_DIR = "context/download-cases-cases"
GROUND_TRUTH = "context/structure-ground-truth-ground_truth.json"
SKILL_FILE = "output/skill.md"
PREDICTIONS_FILE = "output/predictions.json"

with open(SKILL_FILE) as f:
    skill = f.read().strip()

with open(GROUND_TRUTH) as f:
    gt = json.load(f)

training_ids = [c["id"] for c in gt if c.get("split") == "training"]

all_ids = training_ids

system_prompt = (
    "You are a CI failure analyzer. Given a CI failure log, identify the source file to fix.\n"
    f"Rule: {skill}\n"
    'Output ONLY valid JSON: {"file": "<path>"} or {"file": null}'
)

results = []
for case_id in all_ids:
    log_path = os.path.join(CASES_DIR, case_id, "failure_log.txt")
    if not os.path.exists(log_path):
        results.append({"id": case_id, "fix_file": None})
        print(f"  [skip] {case_id}: no log", file=sys.stderr)
        continue

    with open(log_path) as f:
        log = f.read()

    # Truncate very long logs
    if len(log) > 8000:
        log = log[:4000] + "\n...[truncated]...\n" + log[-4000:]

    prompt = f"CI log:\n{log}"

    try:
        result = subprocess.run(
            ["claude", "--print", "--model", "claude-haiku-4-5-20251001",
             "--system-prompt", system_prompt, prompt],
            capture_output=True, text=True, timeout=60
        )
        text = result.stdout.strip()
        m = re.search(r'\{"file"\s*:\s*("(?:[^"\\]|\\.)*"|null)\}', text)
        if m:
            val = m.group(1)
            fix_file = None if val == "null" else val.strip('"')
        else:
            fix_file = None
        results.append({"id": case_id, "fix_file": fix_file})
        print(f"  {case_id}: {fix_file!r}", file=sys.stderr)
    except Exception as e:
        results.append({"id": case_id, "fix_file": None})
        print(f"  [err] {case_id}: {e}", file=sys.stderr)

with open(PREDICTIONS_FILE, "w") as f:
    json.dump({"cases": results}, f, indent=2)

print(f"Wrote {len(results)} predictions to {PREDICTIONS_FILE}")
