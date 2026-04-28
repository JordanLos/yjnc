Skip `[command]/usr/bin/git`.

Exclude: `test*/`, `spec/`, `__tests__/`, `*.{test,spec}.*`, `node_modules/` (if only `node_modules/` errors → `eslint.config.js`).

1. Git diff/Prettier — `--- a/<path>` or `[warn] <path>`.
2. Linter/TS error — `<path>:line:col  error`; strip `runner/work/<repo>/<repo>/` prefix.
3. Stack trace — `at <fn> (<path>:line:col)`; first `lib/` or `src/`.
4. Failing test — `FAIL <test>` → `src/<basename>`.

Output: `{"file": "<path>|null", "reason": "..."}`.
