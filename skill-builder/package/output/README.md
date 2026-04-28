# CI Fault Localization Skill

## What the skill does

Given a CI failure log, predict the **fix file** (`fix_file`) — the source file that needs to be edited to resolve the failure.

The skill reads the failure log and returns:

```json
{"file": "<path>|null", "reason": "..."}
```

It applies a priority-ordered set of signal rules:

1. **Git diff / Prettier** — `--- a/<path>` or `[warn] <path>`
2. **Linter / TypeScript error** — `<path>:line:col  error`; strips CI workspace prefixes; picks the most-cited source file
3. **Stack trace** — `at <fn> (<path>:line:col)`; first `lib/` or `src/` frame
4. **Failing test** — `FAIL <test>` → `src/<basename>`

Test files (`test*/`, `spec/`, `__tests__/`, `*.test.*`, `*.spec.*`) and `node_modules/` are excluded from candidates (unless only `node_modules/` errors appear, in which case `eslint.config.js` is returned).

Git command lines (`[command]/usr/bin/git`) are skipped.

## Benchmark

### Dataset

| Property | Value |
|---|---|
| Total cases | 41 |
| Training split | 31 |
| Held-out split | 10 |
| Repositories | axios, eslint, mocha, prettier, sinon, undici |

Cases were selected from a 60-case corpus (axios, eslint, mocha, prettier, sinon, undici) and filtered to include only those with an unambiguous single-file fix (`fix_diff` touches exactly one production file), supplemented by all 10 held-out cases used in the validation run.

Each case lives in `benchmark/cases/<id>/` and contains:

- `failure_log.txt` — the raw CI log fed to the skill
- `meta.json` — ground-truth fields (`id`, `split`, `fix_file`)
- `fix_diff.json` — the actual patch that fixed the issue (for reference)

### Metric

**Exact-match micro-F1** computed over all cases in the requested split:

- **TP** — predicted `fix_file` equals ground-truth `fix_file` (both non-null, exact string match)
- **FP** — predicted `fix_file` is non-null but differs from ground truth
- **FN** — ground truth `fix_file` is non-null but prediction is null or wrong
- **TN** — both prediction and ground truth are null

Precision = TP / (TP + FP), Recall = TP / (TP + FN), F1 = 2·P·R / (P + R)

### Published scores

These scores are as reported by the minimize/validate pipeline. Running `scorer.sh` against the prediction files in `context/` reproduces them within ±0.01 (minor implementation-level differences in F1 rounding).

| Split | F1 | Cases |
|---|---|---|
| Training | 0.9194 | 31 |
| Held-out (validation) | 0.5000 | 10 |

Token count of `skill.md`: **78**

## How to run the scorer

### Against the full dataset

```bash
bash benchmark/scorer.sh predictions.json benchmark/ground_truth.json all
```

### Against training or held-out only

```bash
bash benchmark/scorer.sh predictions.json benchmark/ground_truth.json training
bash benchmark/scorer.sh predictions.json benchmark/ground_truth.json held-out
```

### Predictions format

`predictions.json` must be either a JSON array or `{"cases": [...]}` where each element is:

```json
{"id": "mocha-001", "fix_file": "lib/mocha.js"}
```

Use `null` for `fix_file` when the skill cannot identify a file.

## Comparing a different skill

1. Run your skill against every `benchmark/cases/<id>/failure_log.txt`
2. Collect predictions into `my_predictions.json`
3. Score: `bash benchmark/scorer.sh my_predictions.json benchmark/ground_truth.json all`
