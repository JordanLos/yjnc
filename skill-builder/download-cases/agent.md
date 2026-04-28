---
name: download-cases
description: Download failure logs and fix diffs for all cases in manifest
model: claude-sonnet-4-6
tools: [Bash, Read, Write]
initialPrompt: "Execute."
---

Read context/case_manifest.json. For each case, download and write structured files.

For each case with id {id}:

1. Download the failure log:
   - Get jobs for the failing run: `gh api "/repos/{repo}/actions/runs/{failing_run_id}/jobs"`
   - Find the failed job(s)
   - Download log: `gh api "/repos/{repo}/actions/jobs/{job_id}/logs"`
   - Extract the relevant section: from first test failure to end of output
   - Trim to 150 lines maximum — keep the stack trace and error, drop runner setup preamble
   - Write to output/cases/{id}/failure_log.txt

2. Download the fix diff:
   - `gh api "/repos/{repo}/compare/{failing_sha}...{passing_sha}"`
   - Extract the changed files and their patches
   - Write to output/cases/{id}/fix_diff.json:
     {"files": ["path/to/fixed.js"], "patch": "...diff content..."}

3. Write output/cases/{id}/meta.json:
   {"id": "{id}", "repo": "{repo}", "split": "training|held-out", "fix_file": "{fix_file}", "error_type": "{error_type}"}

After all cases are downloaded, write output/cases/index.json listing all case ids with their split.

If a case fails to download (run logs expired, API error), skip it and log the reason to output/skipped.json. Compensate by finding a replacement case if more than 5 are skipped.
