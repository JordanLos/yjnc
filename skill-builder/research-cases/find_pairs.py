#!/usr/bin/env python3
"""Find fail→pass CI pairs with single production file fixes and real test stack traces.

Fetches multiple pages per repo and validates pairs have proper test failures.
"""

import json
import subprocess
import sys
from datetime import datetime

WORKFLOWS = [
    # Core 5 repos from spec
    ("eslint/eslint",        27292,      "CI"),
    ("expressjs/express",    10933076,   "ci"),
    ("axios/axios",          226646854,  "Continuous integration"),
    ("axios/axios",          237716929,  "Continuous integration (v0.x)"),
    ("mochajs/mocha",        2830530,    "Tests"),
    ("lodash/lodash",        197066065,  "CI Node.js"),
    # Additional repos to compensate (spec allows this)
    ("prettier/prettier",    439700,     "Prod"),
    ("webpack/webpack",      5647726,    "Github Actions"),
]

TEST_JOB_KEYWORDS = ["test", "spec", "vitest", "jest", "mocha", "jasmine",
                     "smoke", "node.js", "node ", "browser", "full test", "test262"]
NON_TEST_KEYWORDS = ["lint", "format", "verify", "tsc", "typecheck", "type check",
                     "coverage", "build", "compile", "docs", "scorecard", "security",
                     "copilot", "octoguide", "inputs", "resolve-inputs", "prettier",
                     "static check", "yarn validate", "benchmarks"]

def gh_run(cmd, timeout=60):
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return r.stdout, r.returncode
    except subprocess.TimeoutExpired:
        return "", 1

def get_all_runs(repo, workflow_id, status, max_pages=5):
    all_runs = []
    for page in range(1, max_pages + 1):
        cmd = ["gh", "api",
               f"/repos/{repo}/actions/workflows/{workflow_id}/runs?status={status}&per_page=100&page={page}",
               "--jq", ".workflow_runs[] | {id: .id, head_sha: .head_sha, created_at: .created_at}"]
        out, rc = gh_run(cmd)
        if rc != 0 or not out.strip():
            break
        runs = [json.loads(l) for l in out.strip().split("\n") if l.strip()]
        if not runs:
            break
        all_runs.extend(runs)
    return all_runs

def parse_dt(s):
    return datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ")

def get_compare(repo, base_sha, head_sha):
    """Return (prod_files, total_files, total_commits) or None if invalid pair."""
    cmd = ["gh", "api", f"/repos/{repo}/compare/{base_sha}...{head_sha}"]
    out, rc = gh_run(cmd)
    if rc != 0 or not out.strip():
        return None
    try:
        data = json.loads(out)
    except:
        return None
    status = data.get("status", "")
    # "behind" = passing SHA is before failing SHA in git history (wrong direction)
    if status == "behind":
        return None
    total_commits = data.get("total_commits", 0)
    # Too many commits = multi-step fix, unlikely single-file
    if total_commits > 8:
        return None
    files = data.get("files", [])
    return files, total_commits

def is_test_file(path):
    p = path.lower()
    if ".test." in p or ".spec." in p:
        return True
    if p.endswith(".snap"):  # Jest snapshots
        return True
    if (p.endswith(".md") or p.endswith(".txt") or p.endswith(".yml") or
            p.endswith(".yaml") or p.endswith(".json") or p.endswith(".lock") or
            p.endswith(".css") or p.endswith(".html")):
        return True
    parts = p.split("/")
    # Filename itself is "test" or "spec"
    basename_no_ext = parts[-1].rsplit(".", 1)[0]
    if basename_no_ext in ("test", "spec", "tests", "specs"):
        return True
    # Directory check
    if any(seg in ("test", "tests", "__tests__", "fixtures", "docs", "examples",
                    "bench", "benchmarks", "coverage", "e2e", "__mocks__") for seg in parts[:-1]):
        return True
    if "/.github/" in p or p.startswith(".github/"):
        return True
    if "/changelog" in p.lower():
        return True
    return False

def is_production_file(path):
    p = path.lower()
    if not (p.endswith(".js") or p.endswith(".ts") or
            p.endswith(".mjs") or p.endswith(".cjs")):
        return False
    return not is_test_file(p)

def is_test_job(job_name):
    name_lower = job_name.lower()
    if any(kw in name_lower for kw in NON_TEST_KEYWORDS):
        return False
    if any(kw in name_lower for kw in TEST_JOB_KEYWORDS):
        return True
    return False

def get_failing_test_jobs(repo, run_id):
    cmd = ["gh", "api", f"/repos/{repo}/actions/runs/{run_id}/jobs",
           "--jq", ".jobs[] | {id: .id, name: .name, conclusion: .conclusion}"]
    out, rc = gh_run(cmd)
    if rc != 0:
        return [], []
    jobs = [json.loads(l) for l in out.strip().split("\n") if l.strip()]
    failed = [j for j in jobs if j.get("conclusion") == "failure"]
    test_failed = [j for j in failed if is_test_job(j["name"])]
    return test_failed, failed

def get_log_tail(repo, job_id, tail_lines=400):
    cmd = ["gh", "api", f"/repos/{repo}/actions/jobs/{job_id}/logs"]
    out, rc = gh_run(cmd, timeout=60)
    if rc != 0 or not out or "410" in out[:50] or "Server Error" in out[:100]:
        return ""
    lines = out.split("\n")
    return "\n".join(lines[-tail_lines:])

def has_test_failure(log):
    if not log:
        return False
    l, ll = log, log.lower()
    # Vitest
    if "failed tests" in ll and ("×" in l or "✗" in l):
        return True
    if "test files" in ll and "failed" in ll and "passed" in ll:
        return True
    # Jest
    if "test suites:" in ll and "failed" in ll:
        return True
    if "tests:" in ll and "failed" in ll and "passed" in ll:
        return True
    # Mocha
    if "passing" in ll and "failing" in ll:
        return True
    # General JS test runner patterns
    if "assertionerror" in ll:
        return True
    if "typeerror" in ll and "at " in ll:
        return True
    if "referenceerror" in ll:
        return True
    if " failed" in ll and (" passed" in ll or "error" in ll):
        return True
    if "npm err! test failed" in ll or "npm err! errno 1" in ll:
        return True
    if "0 passing" in ll or "1 failing" in ll:
        return True
    return False

def classify_error(log):
    ll = log.lower() if log else ""
    if "typeerror" in ll:
        return "TypeError"
    if "assertionerror" in ll:
        return "AssertionError"
    if "referenceerror" in ll:
        return "ReferenceError"
    if "syntaxerror" in ll:
        return "SyntaxError"
    if "rangeerror" in ll:
        return "RangeError"
    if "expected" in ll and "assert" in ll:
        return "AssertionError"
    return "Error"

def extract_snippet(log, max_chars=500):
    if not log:
        return ""
    lines = log.split("\n")
    interesting = []
    keywords = ["error:", "fail", "×", "✗", "●", "assert", "expect",
                "passing", "failing", "TypeError", "AssertionError"]
    seen = set()
    for i, line in enumerate(lines):
        if any(kw.lower() in line.lower() for kw in keywords):
            for j in range(max(0, i-1), min(len(lines), i+4)):
                if lines[j] not in seen:
                    seen.add(lines[j])
                    interesting.append(lines[j])
    return "\n".join(interesting[:25])[:max_chars]

def process_repo(repo, workflow_id, max_pages=5):
    print(f"\n  [{repo} wf={workflow_id}]", flush=True)
    failed = get_all_runs(repo, workflow_id, "failure", max_pages=max_pages)
    passed = get_all_runs(repo, workflow_id, "success", max_pages=max_pages)
    print(f"    {len(failed)} failed, {len(passed)} passed", flush=True)

    failed.sort(key=lambda x: x["created_at"])
    passed.sort(key=lambda x: x["created_at"])

    # Build pairs: for each failed run, find next passing run
    pairs = []
    for f_run in failed:
        f_dt = parse_dt(f_run["created_at"])
        for p_run in passed:
            if parse_dt(p_run["created_at"]) > f_dt:
                pairs.append({
                    "repo": repo, "workflow_id": workflow_id,
                    "failing_run_id": f_run["id"],
                    "failing_sha": f_run["head_sha"],
                    "failing_dt": f_run["created_at"],
                    "passing_sha": p_run["head_sha"],
                    "passing_run_id": p_run["id"],
                    "passing_dt": p_run["created_at"],
                })
                break

    # Deduplicate by failing_sha (same commit may have multiple trigger events)
    seen_sha = set()
    unique_pairs = []
    for p in pairs:
        if p["failing_sha"] not in seen_sha:
            seen_sha.add(p["failing_sha"])
            unique_pairs.append(p)
    pairs = unique_pairs

    # Filter for single production file change
    good = []
    for pair in pairs:
        result = get_compare(repo, pair["failing_sha"], pair["passing_sha"])
        if result is None:
            continue
        files, total_commits = result
        prod = [f for f in files if is_production_file(f["filename"])]
        if len(prod) == 1:
            pair["fix_file"] = prod[0]["filename"]
            pair["total_files_changed"] = len(files)
            pair["total_commits"] = total_commits
            good.append(pair)
            print(f"    ✓ {pair['fix_file']} ({pair['failing_sha'][:7]}→{pair['passing_sha'][:7]}) [{total_commits}c]", flush=True)

    print(f"    → {len(good)} single-file candidates", flush=True)
    return good

def verify_candidates(candidates):
    verified = []
    skipped = {"no_test_job": 0, "log_expired": 0, "unclear_log": 0}

    for cand in candidates:
        repo = cand["repo"]
        run_id = cand["failing_run_id"]

        test_jobs, _ = get_failing_test_jobs(repo, run_id)
        if not test_jobs:
            skipped["no_test_job"] += 1
            continue

        log = ""
        good_job = None
        for job in test_jobs:
            log = get_log_tail(repo, job["id"], tail_lines=400)
            if log:
                good_job = job
                break

        if not log:
            skipped["log_expired"] += 1
            continue

        if has_test_failure(log):
            cand["error_type"] = classify_error(log)
            cand["job_name"] = good_job["name"] if good_job else "unknown"
            cand["log_snippet"] = extract_snippet(log)
            verified.append(cand)
            print(f"  ✓ {repo} {cand['fix_file']} [{cand['error_type']}]", flush=True)
        else:
            skipped["unclear_log"] += 1
            print(f"  ✗ {repo} {cand['fix_file']} (log unclear)", flush=True)

    print(f"\n  Skipped: no_test_job={skipped['no_test_job']}, "
          f"expired={skipped['log_expired']}, unclear={skipped['unclear_log']}", flush=True)
    return verified

def main():
    all_candidates = []
    print("=== Collecting pairs ===", flush=True)
    for repo, wf_id, wf_name in WORKFLOWS:
        pairs = process_repo(repo, wf_id, max_pages=5)
        all_candidates.extend(pairs)

    print(f"\n=== {len(all_candidates)} raw single-file candidates ===", flush=True)

    print("\n=== Verifying logs ===", flush=True)
    verified = verify_candidates(all_candidates)

    print(f"\n=== Summary ({len(verified)} verified) ===", flush=True)
    by_repo = {}
    for c in verified:
        by_repo.setdefault(c["repo"], []).append(c)
    for repo, cases in sorted(by_repo.items()):
        print(f"  {repo}: {len(cases)}", flush=True)
        for c in cases:
            print(f"    {c['fix_file']} [{c['error_type']}] {c['failing_sha'][:7]}→{c['passing_sha'][:7]}", flush=True)

    with open("output/candidates_full.json", "w") as f:
        json.dump(verified, f, indent=2)
    print(f"\nSaved to output/candidates_full.json", flush=True)
    return verified

if __name__ == "__main__":
    main()
