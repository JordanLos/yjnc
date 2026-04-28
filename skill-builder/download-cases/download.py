#!/usr/bin/env python3
"""Download failure logs and fix diffs for each case in the manifest."""

import json
import subprocess
import os
import re
import sys

MANIFEST_PATH = "context/research-cases-case_manifest.json"
OUTPUT_DIR = "output/cases"
SKIPPED_PATH = "output/skipped.json"
MAX_LOG_LINES = 150

# Patterns that mark the start of a test failure section
FAILURE_PATTERNS = [
    re.compile(r'(FAIL|FAILED|Error:|TypeError:|ReferenceError:|AssertionError:|SyntaxError:|✕|✗|×|● |not ok |TAP version)', re.IGNORECASE),
]

TIMESTAMP_RE = re.compile(r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z ')


def strip_timestamp(line):
    return TIMESTAMP_RE.sub('', line)


def gh_api_json(path):
    result = subprocess.run(
        ['gh', 'api', path, '--paginate'],
        capture_output=True, text=True, timeout=60
    )
    if result.returncode != 0:
        raise Exception(f"API error for {path}: {result.stderr.strip()}")
    # gh --paginate may return multiple JSON objects concatenated; handle arrays
    text = result.stdout.strip()
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        # Try combining paginated array responses
        parts = []
        for chunk in text.split('\n['):
            chunk = chunk.strip()
            if chunk and not chunk.startswith('['):
                chunk = '[' + chunk
            try:
                parts.extend(json.loads(chunk))
            except Exception:
                pass
        if parts:
            return parts
        raise


def gh_api_text(path):
    result = subprocess.run(
        ['gh', 'api', path],
        capture_output=True, text=True, timeout=120
    )
    if result.returncode != 0:
        raise Exception(f"Log API error for {path}: {result.stderr.strip()}")
    return result.stdout


def extract_failure_section(raw_log):
    lines = raw_log.split('\n')
    # Strip timestamps
    lines = [strip_timestamp(l) for l in lines]

    start_idx = None
    for i, line in enumerate(lines):
        for pat in FAILURE_PATTERNS:
            if pat.search(line):
                # Back up a few lines for context
                start_idx = max(0, i - 3)
                break
        if start_idx is not None:
            break

    if start_idx is None:
        # No failure marker found; take last MAX_LOG_LINES
        relevant = lines[-MAX_LOG_LINES:]
    else:
        relevant = lines[start_idx:]
        if len(relevant) > MAX_LOG_LINES:
            relevant = relevant[:MAX_LOG_LINES]

    return '\n'.join(relevant)


def process_case(case):
    case_id = case['id']
    repo = case['repo']
    failing_run_id = case['failing_run_id']
    failing_sha = case['failing_sha']
    passing_sha = case['passing_sha']

    out_dir = os.path.join(OUTPUT_DIR, case_id)
    os.makedirs(out_dir, exist_ok=True)

    # --- Step 1: failure log ---
    jobs_data = gh_api_json(f'/repos/{repo}/actions/runs/{failing_run_id}/jobs')
    jobs = jobs_data.get('jobs', jobs_data) if isinstance(jobs_data, dict) else jobs_data

    failed_jobs = [j for j in jobs if j.get('conclusion') == 'failure']
    if not failed_jobs:
        failed_jobs = [j for j in jobs if j.get('conclusion') not in ('success', 'skipped', None)]
    if not failed_jobs:
        raise Exception(f"No failed jobs in run {failing_run_id}")

    job_id = failed_jobs[0]['id']
    raw_log = gh_api_text(f'/repos/{repo}/actions/jobs/{job_id}/logs')
    failure_log = extract_failure_section(raw_log)

    with open(os.path.join(out_dir, 'failure_log.txt'), 'w') as f:
        f.write(failure_log)

    # --- Step 2: fix diff ---
    compare = gh_api_json(f'/repos/{repo}/compare/{failing_sha}...{passing_sha}')
    files = []
    patch_parts = []
    for fobj in compare.get('files', []):
        files.append(fobj['filename'])
        if 'patch' in fobj:
            patch_parts.append(
                f"--- a/{fobj['filename']}\n+++ b/{fobj['filename']}\n{fobj['patch']}"
            )

    fix_diff = {'files': files, 'patch': '\n'.join(patch_parts)}
    with open(os.path.join(out_dir, 'fix_diff.json'), 'w') as f:
        json.dump(fix_diff, f, indent=2)

    # --- Step 3: meta ---
    meta = {
        'id': case_id,
        'repo': repo,
        'split': case['split'],
        'fix_file': case['fix_file'],
        'error_type': case['error_type'],
    }
    with open(os.path.join(out_dir, 'meta.json'), 'w') as f:
        json.dump(meta, f, indent=2)


def main():
    with open(MANIFEST_PATH) as f:
        cases = json.load(f)

    os.makedirs(OUTPUT_DIR, exist_ok=True)

    skipped = []
    completed = []

    for i, case in enumerate(cases, 1):
        case_id = case['id']
        print(f"[{i}/{len(cases)}] {case_id} ({case['repo']})...", end=' ', flush=True)
        try:
            process_case(case)
            completed.append(case_id)
            print("OK")
        except Exception as e:
            reason = str(e)
            print(f"SKIP — {reason}")
            skipped.append({'id': case_id, 'repo': case['repo'], 'reason': reason})

    # Write index.json
    completed_set = set(completed)
    index = [
        {'id': c['id'], 'split': c['split']}
        for c in cases
        if c['id'] in completed_set
    ]
    with open(os.path.join(OUTPUT_DIR, 'index.json'), 'w') as f:
        json.dump(index, f, indent=2)

    # Write skipped.json (always write, even if empty)
    with open(SKIPPED_PATH, 'w') as f:
        json.dump(skipped, f, indent=2)

    print(f"\nDone: {len(completed)} completed, {len(skipped)} skipped")
    if len(skipped) > 5:
        print(f"WARNING: {len(skipped)} cases skipped — consider finding replacements")


if __name__ == '__main__':
    main()
