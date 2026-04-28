#!/usr/bin/env python3
"""Collect fail->pass CI training cases per the spec.

Strategy:
  1. Fetch recent failing runs (all branches) per repo.
  2. Find next passing run on same (branch, workflow_id).
  3. Require compare.total_commits <= 8 AND single production file changed
     AND <=500 lines changed. This enforces tight "broken->fix" pairs.
  4. Fetch failure job log. Enforce signal_check + skip_reason.
  5. Classify error_type; record.

Parallelism: up to 6 worker threads on per-pair evaluation (compare + jobs + log).

Writes:
  output/candidates.json   running list
  output/case_manifest.json final selection
"""

import json
import os
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from threading import Lock

OUTPUT_DIR = "/Users/jordan/code/juc/skill-builder/research-cases/output"
CANDIDATES_PATH = os.path.join(OUTPUT_DIR, "candidates.json")
MANIFEST_PATH = os.path.join(OUTPUT_DIR, "case_manifest.json")

TARGET_REPOS = [
    # Only repos that produced hits in round 1
    ("mochajs/mocha",       "mocha"),
    ("eslint/eslint",       "eslint"),
    ("sinonjs/sinon",       "sinon"),
    ("axios/axios",         "axios"),
    ("prettier/prettier",   "prettier"),
    ("nodejs/undici",       "undici"),
]

TEST_PATH_MARKERS = ["test/", "tests/", "__tests__", ".spec.", ".test.",
                     "docs/", "examples/", "fixtures/", ".github/",
                     "node_modules/", "benchmark/", "bench/", "coverage/"]


def gh(path, timeout=60):
    cmd = ["gh", "api", path]
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return r.stdout, r.returncode
    except subprocess.TimeoutExpired:
        return "", 1


def gh_json(path, timeout=60):
    out, rc = gh(path, timeout)
    if rc != 0 or not out.strip():
        return None
    try:
        return json.loads(out)
    except json.JSONDecodeError:
        return None


def parse_dt(s):
    return datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)


def is_production_file(path):
    p = path.lower()
    if not (p.endswith(".js") or p.endswith(".ts") or p.endswith(".mjs") or p.endswith(".cjs")):
        return False
    if p.endswith(".d.ts"):
        return False
    for marker in TEST_PATH_MARKERS:
        if marker in p:
            return False
    if p.startswith("lib/") or p.startswith("src/"):
        return True
    parts = p.split("/")
    if len(parts) >= 3 and parts[0] == "packages" and parts[2] in ("lib", "src"):
        return True
    if "/" not in p:
        return True
    return False


def load_candidates():
    if os.path.exists(CANDIDATES_PATH):
        with open(CANDIDATES_PATH) as f:
            try:
                return json.load(f)
            except json.JSONDecodeError:
                return []
    return []


_save_lock = Lock()


def save_candidates(cands):
    with _save_lock:
        tmp = CANDIDATES_PATH + ".tmp"
        with open(tmp, "w") as f:
            json.dump(cands, f, indent=2)
        os.replace(tmp, CANDIDATES_PATH)


def fetch_failed_runs(repo, per_page=100, max_pages=8):
    runs = []
    seen = set()
    for page in range(1, max_pages + 1):
        path = f"/repos/{repo}/actions/runs?status=failure&per_page={per_page}&page={page}"
        data = gh_json(path)
        if not data:
            break
        page_runs = data.get("workflow_runs", [])
        if not page_runs:
            break
        for r in page_runs:
            if r["id"] in seen:
                continue
            seen.add(r["id"])
            runs.append({
                "id": r["id"],
                "head_sha": r["head_sha"],
                "head_branch": r.get("head_branch") or "",
                "workflow_id": r["workflow_id"],
                "workflow_name": r.get("name", ""),
                "created_at": r["created_at"],
            })
        if len(page_runs) < per_page:
            break
    return runs


def find_next_passing(repo, branch, workflow_id, failing_created_at):
    if not branch:
        return None
    path = (f"/repos/{repo}/actions/runs?status=success"
            f"&branch={branch}&per_page=100")
    data = gh_json(path)
    if not data:
        return None
    runs = [r for r in data.get("workflow_runs", [])
            if r.get("workflow_id") == workflow_id]
    failing_dt = parse_dt(failing_created_at)
    cands = [r for r in runs if parse_dt(r["created_at"]) > failing_dt]
    if not cands:
        return None
    cands.sort(key=lambda x: x["created_at"])
    r = cands[0]
    return {"id": r["id"], "head_sha": r["head_sha"], "created_at": r["created_at"]}


def get_compare(repo, base_sha, head_sha):
    return gh_json(f"/repos/{repo}/compare/{base_sha}...{head_sha}")


def get_failed_jobs(repo, run_id):
    data = gh_json(f"/repos/{repo}/actions/runs/{run_id}/jobs?per_page=100")
    if not data:
        return []
    return [j for j in data.get("jobs", []) if j.get("conclusion") == "failure"]


def get_log_first(repo, job_id, nbytes=50_000):
    out, rc = gh(f"/repos/{repo}/actions/jobs/{job_id}/logs", timeout=90)
    if rc != 0 or not out:
        return ""
    return out[:nbytes]


SKIP_PATTERNS = [
    ("Cannot find module", "module_not_found"),
    ("MODULE_NOT_FOUND", "module_not_found"),
    ("ENOTFOUND", "ci_network"),
    ("getaddrinfo EAI_AGAIN", "ci_network"),
]


def signal_check(log, fix_file):
    if not log or not fix_file:
        return False
    if fix_file in log:
        return True
    basename = fix_file.rsplit("/", 1)[-1]
    for line in log.split("\n"):
        if basename in line and ("at " in line or "file:///" in line or "❯" in line):
            return True
    return False


def skip_reason(log):
    for pat, reason in SKIP_PATTERNS:
        if pat in log:
            return reason
    return None


ERROR_ORDER = ["TypeError", "AssertionError", "ReferenceError",
               "RangeError", "SyntaxError", "Error"]


def classify_error(log):
    for err in ERROR_ORDER:
        if err in log:
            return err
    return "Error"


def evaluate_pair(repo, run, passing):
    """Return a dict: either {"status":"pass",...} or {"status":"fail","reject_reason":...}."""
    out = {
        "repo": repo,
        "failing_run_id": run["id"],
        "failing_sha": run["head_sha"],
        "passing_sha": passing["head_sha"],
        "fix_file": None,
        "error_type": None,
        "status": "fail",
        "reject_reason": None,
        "split": None,
    }
    if run["head_sha"] == passing["head_sha"]:
        out["reject_reason"] = "same_sha_rerun"
        return out
    cmp_data = get_compare(repo, run["head_sha"], passing["head_sha"])
    if not cmp_data:
        out["reject_reason"] = "compare_404"
        return out
    if cmp_data.get("status") == "behind":
        out["reject_reason"] = "passing_behind_failing"
        return out
    total_commits = cmp_data.get("total_commits", 0)
    if total_commits > 8:
        out["reject_reason"] = f"too_many_commits({total_commits})"
        return out

    files = cmp_data.get("files", [])
    total_lines = sum(f.get("changes", 0) for f in files)
    if total_lines > 500:
        out["reject_reason"] = f"diff_too_large({total_lines})"
        return out

    prod_files = [f for f in files
                  if is_production_file(f["filename"])
                  and f.get("status") in ("modified", "added")]
    if len(prod_files) == 0:
        out["reject_reason"] = "no_prod_file"
        return out
    if len(prod_files) > 1:
        out["reject_reason"] = "multi_prod_file"
        return out

    fix_file = prod_files[0]["filename"]
    out["fix_file"] = fix_file

    failed_jobs = get_failed_jobs(repo, run["id"])
    if not failed_jobs:
        out["reject_reason"] = "no_failed_jobs"
        return out

    reason = None
    for job in failed_jobs[:6]:
        log = get_log_first(repo, job["id"])
        if not log:
            reason = "log_unavailable"
            continue
        sr = skip_reason(log)
        if sr:
            reason = sr
            continue
        if not signal_check(log, fix_file):
            reason = "no_signal"
            continue
        out["status"] = "pass"
        out["error_type"] = classify_error(log)
        out["reject_reason"] = None
        return out

    out["reject_reason"] = reason or "no_qualifying_log"
    return out


def process_repo(repo, short, existing_all, target_per_repo=8, max_checks=40):
    print(f"\n=== {repo} ===", flush=True)
    existing_fail_shas = {c["failing_sha"] for c in existing_all if c["repo"] == repo}
    failed_runs = fetch_failed_runs(repo, per_page=100, max_pages=8)
    print(f"  fetched {len(failed_runs)} failed runs", flush=True)

    # Dedup by failing_sha and already-seen
    unique = []
    seen = set(existing_fail_shas)
    for r in failed_runs:
        if r["head_sha"] in seen:
            continue
        seen.add(r["head_sha"])
        unique.append(r)
    print(f"  {len(unique)} new unique failing SHAs", flush=True)

    # Phase 1: find next passing in parallel
    pairs = []
    def _find(run):
        np = find_next_passing(repo, run["head_branch"], run["workflow_id"],
                               run["created_at"])
        return (run, np)

    with ThreadPoolExecutor(max_workers=6) as ex:
        futures = [ex.submit(_find, r) for r in unique[:max_checks * 4]]
        for fut in as_completed(futures):
            run, np = fut.result()
            if np is None:
                continue
            pairs.append({"failing": run, "passing": np})

    print(f"  {len(pairs)} pairs have a next-passing run", flush=True)

    # Phase 2: evaluate pairs in parallel, capped at max_checks
    pairs = pairs[:max_checks]
    new_cands = []

    def _eval(pair):
        return evaluate_pair(repo, pair["failing"], pair["passing"])

    keeps = 0
    with ThreadPoolExecutor(max_workers=6) as ex:
        futures = [ex.submit(_eval, p) for p in pairs]
        for fut in as_completed(futures):
            res = fut.result()
            new_cands.append(res)
            tag = "KEEP" if res["status"] == "pass" else f"fail({res['reject_reason']})"
            ff = res.get("fix_file") or "-"
            print(f"    {res['failing_sha'][:7]} -> {res['passing_sha'][:7]} "
                  f"[{tag}] {ff} {res.get('error_type') or ''}", flush=True)
            if res["status"] == "pass":
                keeps += 1
            # Persist every 5 new records
            if len(new_cands) % 5 == 0:
                save_candidates(existing_all + new_cands)
            if keeps >= target_per_repo:
                # We have enough; wait for pending but don't queue more
                pass

    print(f"  kept={keeps} (out of {len(new_cands)} evaluated)", flush=True)
    return new_cands, len(new_cands)


def assemble_manifest(all_cands, target_total=60, holdout=10):
    kept = [c for c in all_cands if c["status"] == "pass"]
    by_repo = {}
    for c in kept:
        by_repo.setdefault(c["repo"], []).append(c)
    selected = []
    active = True
    while active and len(selected) < target_total:
        active = False
        for repo in list(by_repo.keys()):
            if by_repo[repo]:
                selected.append(by_repo[repo].pop(0))
                active = True
                if len(selected) >= target_total:
                    break

    held_out_idx = set()
    if len(selected) >= holdout:
        step = len(selected) / holdout
        for i in range(holdout):
            held_out_idx.add(min(int(i * step + step / 2), len(selected) - 1))

    repo_short = {r: s for r, s in TARGET_REPOS}
    per_repo_count = {}
    manifest = []
    for idx, c in enumerate(selected):
        short = repo_short.get(c["repo"], "unknown")
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
    return manifest


def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    all_cands = load_candidates()
    print(f"Loaded {len(all_cands)} existing candidates "
          f"({sum(1 for c in all_cands if c['status']=='pass')} passing)", flush=True)

    summary = {}
    for repo, short in TARGET_REPOS:
        kept_before = sum(1 for c in all_cands
                         if c["repo"] == repo and c["status"] == "pass")
        # No skip shortcut — we want more from productive repos
        target = 30
        new_cands, checked = process_repo(repo, short, all_cands,
                                          target_per_repo=target,
                                          max_checks=150)
        all_cands.extend(new_cands)
        save_candidates(all_cands)
        kept_after = sum(1 for c in all_cands
                        if c["repo"] == repo and c["status"] == "pass")
        summary[repo] = {"checked": checked, "kept": kept_after}

        total_kept = sum(1 for c in all_cands if c["status"] == "pass")
        print(f"  -- total kept so far: {total_kept}", flush=True)
        if total_kept >= 70:
            print("Reached 70 kept cases, stopping.", flush=True)
            break

    manifest = assemble_manifest(all_cands, target_total=60, holdout=10)
    with open(MANIFEST_PATH, "w") as f:
        json.dump(manifest, f, indent=2)
    save_candidates(all_cands)

    print("\n=== SUMMARY ===", flush=True)
    total_checked = 0
    total_kept = 0
    for repo, stats in summary.items():
        print(f"  {repo}: checked={stats['checked']}, kept={stats['kept']}", flush=True)
        total_checked += stats["checked"]
        total_kept += stats["kept"]
    print(f"TOTAL: checked={total_checked}, kept={total_kept}", flush=True)
    print(f"Manifest size: {len(manifest)}", flush=True)


if __name__ == "__main__":
    main()
