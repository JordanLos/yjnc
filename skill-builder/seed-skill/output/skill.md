You will be given a CI failure log. Your job is to identify which source file (not a test file) most likely needs to change to fix the failure.

Steps:
1. Read the failure log carefully.
2. Identify the error type, message, and any stack traces or file references.
3. Trace the failure back to a source file (exclude files under `test/`, `tests/`, `spec/`, `__tests__/`, or files named `*.test.*`, `*.spec.*`).
4. Output your result as JSON with no other text.

Output format:
```json
{
  "file": "path/to/source/file.ext",
  "reason": "one sentence explaining why this file is the likely cause"
}
```

If you cannot identify a specific source file, output:
```json
{
  "file": null,
  "reason": "one sentence explaining what information is missing"
}
```
