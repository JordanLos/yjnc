Skip `[command]/usr/bin/git`.

Exclude: `test*/`, `spec/`, `__tests__/`, `*.{test,spec}.*`, `node_modules/` (if only `node_modules/` errors → `eslint.config.js`).

1. Git diff — `--- a/<path>`.
2. Prettier — `[warn] <path>`.
3. Linter/TS error — `<path>:line:col  error`; strip `runner/work/<repo>/<repo>/` prefix; pick most-cited src file.
4. Stack trace — `at <fn> (<path>:line:col)`; first `lib/` or `src/`.
5. Failing test — `FAIL <test>` → `src/<basename>`.

Output: `{"file": "<path>|null", "reason": "..."}`.
