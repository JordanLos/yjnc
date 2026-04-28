You will be given a CI failure log. Your job is to identify which source file (not a test file) most likely needs to change to fix the failure.

Steps:
1. Read the failure log carefully, ignoring git cleanup boilerplate and `[command]/usr/bin/git` lines.
2. Identify the error type, message, and any stack traces or file references.
3. Use the signal priority order below to select the fix file.
4. Trace the failure back to a source file (exclude files under `test/`, `tests/`, `spec/`, `__tests__/`, or files named `*.test.*`, `*.spec.*`).
5. Output your result as JSON with no other text.

Signal priority (use the highest-priority signal present):

1. **Git diff header** — if the log contains `--- a/path/to/file`, that file is the fix file (pick the first non-test file).

2. **Prettier `[warn]` lines** — if the log contains lines like `[warn] path/to/file.js` (with or without ANSI color codes, e.g. `[\x1b[33mwarn\x1b[39m] path/to/file.js`), strip any ANSI escape sequences and treat it as a `[warn]` line. The first non-test file listed is the fix file. Ignore lines like `[warn] Code style issues found…`.

3. **Linter error with file path** — lines like `path/file.js:line:col  error …` or `##[error]path/file.js(line,col): error …`. Strip any CI runner prefix (`/home/runner/work/<repo>/<repo>/`) to get the relative path. Pick the first non-test source file. Ignore paths under `node_modules/`.

4. **TypeScript error** — `##[error]src/path/file.js(line,col): error TS…` → strip `##[error]` to get the path.

5. **Stack trace** — lines like `at functionName (path/file.js:line:col)`. Pick the first `lib/` or `src/` entry that is not a test file.
   - Convert Windows backslashes to forward slashes.
   - If the stack references a bundled single-filename `.js` (e.g. `mocha.js`, `bundle.js`) with a function like `requireX (mocha.js:N)`, infer the source as `lib/X.js` where `X` is the lowercase name after `require`.

6. **Failing test file** — if a test file name is shown (e.g. `FAIL tests/unit/foo.js`), the source fix is the file with the same base name under `src/` (e.g. `src/.../foo.js`). Use the base name match.

Constraints:
- Never return a path containing `node_modules/`.
- Never return a test file path.
- If multiple files are listed, prefer the one that appears most often or is listed first (for linter errors), unless a git diff is present (which is definitive).
- When the only visible error is about a `node_modules` JSON file (e.g. ESLint failing to parse a dependency's `package.json`, tagged as `directory description file`), this indicates an ESLint configuration problem — return `eslint.config.js` as the fix file (the config needs to ignore node_modules). Do not return null for this pattern.

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

---

## Worked Examples

### Example 1 — ANSI-colored Prettier `[warn]` line

Log (excerpt):
```
[\x1b[33mwarn\x1b[39m] lib/sinon/sandbox.js
[\x1b[33mwarn\x1b[39m] Code style issues found in the above file. Run Prettier with --write to fix.
##[error]Process completed with exit code 1.
```

After stripping ANSI codes these read `[warn] lib/sinon/sandbox.js` and `[warn] Code style issues found…`. Signal priority 2 applies; `lib/sinon/sandbox.js` is a non-test source file.

Output:
```json
{"file": "lib/sinon/sandbox.js", "reason": "Prettier [warn] line (ANSI-stripped) identifies lib/sinon/sandbox.js as having code style issues."}
```

### Example 2 — ESLint linting node_modules JSON files

Log (excerpt):
```
/home/runner/work/mocha/mocha/docs/_data/supporters.cjs
##[error]   20:23  error  /home/runner/work/mocha/mocha/docs/node_modules/debug/package.json (directory description file): SyntaxError: Unexpected end of JSON input  n/no-missing-require
✖ 2 problems (2 errors, 0 warnings)
ERROR: "lint:code" exited with 1.
```

All errors reference `node_modules/*.json` tagged as `directory description file`. Apply the node_modules constraint: ESLint config needs to exclude node_modules.

Output:
```json
{"file": "eslint.config.js", "reason": "ESLint is linting node_modules/debug/package.json, so eslint.config.js is missing a rule to ignore node_modules."}
```
