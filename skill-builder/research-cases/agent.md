---
name: research-cases
description: Select CI failure cases where the fix_file is identifiable from the failure log
model: claude-sonnet-4-6
tools: [Bash, Write]
initialPrompt: "Execute."
---

Find 60 JavaScript CI failure cases where the failure log contains a direct stack trace reference to the fix_file. Quality over quantity — skip any case that fails the signal check.

**Target repos** (prefer simple libraries with direct-import unit tests, not build-system integration tests):
- eslint/eslint
- mochajs/mocha
- sinonjs/sinon
- chaijs/chai
- yargs/yargs
- fastify/fastify
- hapijs/joi
- expressjs/express
- date-fns/date-fns
- karma-runner/karma

For each repo:
1. List recent failed runs: `gh api "/repos/{repo}/actions/runs?status=failure&per_page=30"`
2. For each failed run, find the next passing run on the same branch/workflow
3. Check the fix diff: `gh api "/repos/{repo}/compare/{failing_sha}...{passing_sha}"`
   - Identify the production fix_file: a `.js/.ts/.mjs/.cjs` file NOT in test dirs, NOT in docs/examples/.github/
   - Skip if more than one production file changed
   - Skip if fix_file is a test file (path contains test/tests/__tests__/.spec./.test.)
4. Download a sample of the failure log: `gh api "/repos/{repo}/actions/runs/{run_id}/jobs"` → get failed job → `gh api "/repos/{repo}/actions/jobs/{job_id}/logs"`

**Signal check (required — skip the case if it fails):**
Extract the fix_file basename (e.g. `suite.js` from `lib/suite.js`).
The failure log MUST contain EITHER:
- The full fix_file path (e.g. `lib/suite.js`) somewhere in the output, OR
- The basename appearing in a stack frame line (lines with `at `, `file:///` paths, or `❯` vitest frames)

**Skip these failure types regardless:**
- "Cannot find module" / module-not-found (Node version compat, not a source bug)
- Linting or formatting failures (output only shows style violations)
- Build/compilation failures where stack only shows compiled bundle paths (`webpack:/`, `dist/`, `build/`)
- CI setup failures (missing env vars, network errors, missing credentials)
- Snapshot/fixture mismatches where only test infrastructure files appear in the stack
- Failures in test infrastructure only (mocha.js, jest-circus, jasmine — not project source)

**Selection criteria:**
- Diverse error types: TypeError, AssertionError, ReferenceError, Error
- At least 4 different repos represented
- Real test failures with stack traces pointing into project source files
- fix_file is a source file in lib/, src/, packages/*/lib/, or root-level .js/.ts

Select 50 training cases and 10 held-out cases. Distribute held-out across repos and error types.

If a repo doesn't have enough qualifying cases after signal check, move to the next repo on the list. Note how many candidates were checked vs kept per repo.

Write output/case_manifest.json:
[
  {
    "id": "eslint-001",
    "repo": "eslint/eslint",
    "failing_run_id": 123456789,
    "failing_sha": "abc1234",
    "passing_sha": "def5678",
    "fix_file": "lib/config/config.js",
    "error_type": "AssertionError",
    "split": "training"
  }
]

Write output/candidates.json incrementally as you evaluate each candidate (running list with pass/fail reason) so progress is visible mid-run.
