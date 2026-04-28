#!/usr/bin/env python3
"""Build case_manifest.json from candidates.json.

Select up to 60 pass entries, split 50 training + 10 held-out,
round-robin across repos for diversity.
"""

import json
import os

CANDIDATES = "/Users/jordan/code/juc/skill-builder/research-cases/output/candidates.json"
MANIFEST = "/Users/jordan/code/juc/skill-builder/research-cases/output/case_manifest.json"

REPO_SHORT = {
    "mochajs/mocha":      "mocha",
    "chaijs/chai":        "chai",
    "sinonjs/sinon":      "sinon",
    "yargs/yargs":        "yargs",
    "eslint/eslint":      "eslint",
    "fastify/fastify":    "fastify",
    "hapijs/joi":         "joi",
    "expressjs/express":  "express",
    "date-fns/date-fns":  "date-fns",
    "karma-runner/karma": "karma",
    "axios/axios":        "axios",
    "lodash/lodash":      "lodash",
    "prettier/prettier":  "prettier",
    "nodejs/undici":      "undici",
    "storybookjs/storybook": "storybook",
    "vercel/next.js":     "next",
}


def main():
    with open(CANDIDATES) as f:
        cands = json.load(f)
    passes = [c for c in cands if c["status"] == "pass"]

    # Round-robin across repos for diversity
    by_repo = {}
    for c in passes:
        by_repo.setdefault(c["repo"], []).append(c)

    target_total = min(60, len(passes))
    selected = []
    while len(selected) < target_total:
        progressed = False
        for repo in list(by_repo.keys()):
            if by_repo[repo]:
                selected.append(by_repo[repo].pop(0))
                progressed = True
                if len(selected) >= target_total:
                    break
        if not progressed:
            break

    # Distribute held-out evenly (10 out of 60, or fewer if less data)
    holdout = min(10, max(1, len(selected) // 6))
    held_out_idx = set()
    if len(selected) >= holdout:
        step = len(selected) / holdout
        for i in range(holdout):
            held_out_idx.add(min(int(i * step + step / 2), len(selected) - 1))

    per_repo_count = {}
    manifest = []
    for idx, c in enumerate(selected):
        short = REPO_SHORT.get(c["repo"], "unknown")
        per_repo_count[short] = per_repo_count.get(short, 0) + 1
        entry = {
            "id": f"{short}-{per_repo_count[short]:03d}",
            "repo": c["repo"],
            "failing_run_id": c["failing_run_id"],
            "failing_sha": c["failing_sha"],
            "passing_sha": c["passing_sha"],
            "fix_file": c["fix_file"],
            "error_type": c["error_type"],
            "split": "held-out" if idx in held_out_idx else "training",
        }
        manifest.append(entry)
        c["split"] = entry["split"]

    with open(MANIFEST, "w") as f:
        json.dump(manifest, f, indent=2)

    # Update candidates.json too so split is recorded there
    with open(CANDIDATES, "w") as f:
        json.dump(cands, f, indent=2)

    print(f"Manifest: {len(manifest)} entries")
    from collections import Counter
    print("repos:", Counter(m["repo"] for m in manifest))
    print("errors:", Counter(m["error_type"] for m in manifest))
    print("split:", Counter(m["split"] for m in manifest))


if __name__ == "__main__":
    main()
