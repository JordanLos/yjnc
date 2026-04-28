---
name: structure-ground-truth
description: Extract structured ground truth from downloaded cases
model: claude-sonnet-4-6
tools: [Read, Write, Bash]
---

Read each case from context/cases/. For each case directory containing failure_log.txt, fix_diff.json, and meta.json:

Extract from failure_log.txt:
- failing_test: the name of the test that failed (from "failing" or "Error in" lines)
- error_type: the exception class (AssertionError, TypeError, etc.)
- file: the source file referenced in the top of the stack trace

Extract from fix_diff.json:
- fix_file: the production file changed in the fix (already in meta.json, verify it matches)

Write output/ground_truth.json — array of objects, training cases first then held-out:
[
  {
    "id": "eslint-001",
    "split": "training",
    "failing_test": "should validate rule config",
    "error_type": "AssertionError",
    "file": "lib/config/config.js",
    "fix_file": "tools/config-rule.js"
  }
]

Exclude a case and mark it excluded: true if:
- fix_diff shows multiple production files changed
- failure_log has no identifiable stack trace
- fix_file is a test file

Write the count of included vs excluded cases to output/summary.json.
