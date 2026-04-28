#!/usr/bin/env python3
"""Process case directories and generate ground_truth.json and summary.json."""

import json
import os
import re

CASES_DIR = "context/download-cases-cases"
OUTPUT_DIR = "output"

ANSI = re.compile(r"\x1b\[[0-9;]*m")

TEST_PATH_RE = re.compile(
    r"(?:^|/)tests?/"
    r"|(?:^|/)specs?/"
    r"|(?:^|/)__tests__/"
    r"|\.(?:test|spec)\.[a-z]+$"
    r"|_test\.[a-z]+$"
    r"|__snapshots__"
    r"|/smoke/"
)

LOCK_FILE_RE = re.compile(
    r"(?:^|/)(?:package-lock|yarn\.lock|pnpm-lock)(?:\.[a-z]+)?$"
)


def strip_ansi(text):
    return ANSI.sub("", text)


def is_test_file(path):
    return bool(TEST_PATH_RE.search(path.replace("\\", "/")))


def is_lock_file(path):
    return bool(LOCK_FILE_RE.search(path.replace("\\", "/")))


def count_production_files(files):
    """Count non-test, non-lock files in the diff."""
    return sum(1 for f in files if not is_test_file(f) and not is_lock_file(f))


def project_relative(path):
    """Strip /home/runner/work/owner/repo/ or similar CI prefix, return relative path."""
    if not path:
        return None
    path = path.replace("\\", "/")
    # node internals
    if re.match(r"(?:node:|internal/)", path):
        return None
    if "node_modules" in path:
        return None
    # runner temp dirs
    if "/runner/work/_temp/" in path or "/github/runner_temp/" in path:
        return None
    # /home/runner/work/{owner}/{repo}/... or /Users/runner/work/...
    m = re.search(r"/runner/work/[^/]+/[^/]+/(.+)", path)
    if m:
        return m.group(1)
    # Windows CI: D:/a/{owner}/{repo}/...
    m = re.search(r"[A-Z]:/a/[^/]+/[^/]+/(.+)", path)
    if m:
        return m.group(1)
    # Already relative
    if not path.startswith("/") and not re.match(r"[A-Z]:", path):
        return path
    return None


def has_identifiable_error(log_content):
    """Return True if the log has a real error beyond just CI boilerplate."""
    clean = strip_ansi(log_content)

    # Real JS/Node stack trace frames
    if re.search(r"^\s{4}at\s+\S", clean, re.MULTILINE):
        return True

    # Exception during run (mocha runner)
    if "Exception during run:" in clean:
        return True

    # Named test failure (mocha, jest, pytest)
    if re.search(r"\d+ failing", clean):
        return True
    if re.search(r"^\s*FAIL\b", clean, re.MULTILINE):
        return True

    # ESLint errors with line:col (CI annotation format)
    if re.search(r"##\[error\]\s+\d+:\d+\s+error\s+", clean):
        return True

    # TypeScript compile errors
    if re.search(r"##\[error\].+\(\d+,\d+\):\s+error\s+TS\d+", clean):
        return True

    # ESLint "  line:col  error  ..." format without ##[error] prefix
    if re.search(r"^\s+\d+:\d+\s+error\s+\S", clean, re.MULTILINE):
        return True

    # Build errors referencing source files (guard against node_modules)
    for line in clean.split("\n"):
        if "node_modules" in line:
            continue
        if re.search(r"(src|lib)/[\w/.-]+\.\w+:\d+:\d+:?\s*(error|ERROR)", line):
            return True
    if re.search(r"ERROR:\s+Could not resolve", clean):
        return True

    # Error: / TypeError: / AssertionError: etc. (not in boilerplate)
    for line in clean.split("\n"):
        if re.search(r"\b(AssertionError|TypeError|ReferenceError|SyntaxError)[\s:\[]", line):
            if "Process completed" not in line and "node_modules" not in line:
                return True

    # Prettier/mocha format check: [warn] file (specific file warning)
    if re.search(r"\[warn\]\s+(?:src|lib)/\S+\.(js|ts|mjs|cjs)", clean):
        return True
    if re.search(r"\[warn\]\s+lib/\S+", clean):
        return True

    # ESLint inline violations (✘ U+2718 or ✖ U+2716 styles)
    if re.search(r"[✘✖]\s+\d+ problem", clean):
        return True

    # ESLint ✘ rule block: the rule name line followed eventually by file:line
    if re.search(r"[✘✖]\s+\w[\w/.-]+", clean):
        # Also check there's a src/lib file path nearby
        if re.search(r"(?:src|lib)/[\w/.-]+\.\w+:\d+:\d+", clean):
            return True

    # Autofix CI: git showed a staged file path (src/... or lib/...)
    for line in clean.split("\n"):
        stripped = line.strip()
        if re.match(r"^(?:src|lib)/[\w/.-]+\.(js|ts|mjs|cjs)$", stripped):
            return True

    return False


def extract_failing_test(log_content):
    """Extract the first failing test name from 'N failing'/'Error in'/FAIL lines."""
    clean = strip_ansi(log_content)
    lines = clean.split("\n")

    # Mocha numbered format: look for "N failing" then find the first "N) Suite" block
    for i, line in enumerate(lines):
        if re.search(r"\d+ failing", line):
            # Scan ahead for "  1) SuiteName\n       TestName:"
            for j in range(i + 1, min(i + 15, len(lines))):
                m = re.match(r"^\s{1,6}\d+\)\s+(.+)$", lines[j])
                if m:
                    parts = [m.group(1).rstrip(":").strip()]
                    # Next non-empty line may be sub-description
                    for k in range(j + 1, min(j + 8, len(lines))):
                        nxt = lines[k].strip()
                        if not nxt:
                            continue
                        if re.match(
                            r"^(Error|AssertionError|TypeError|ReferenceError|Uncaught|\[|\d+\))",
                            nxt,
                        ):
                            break
                        parts.append(nxt.rstrip(":"))
                        break
                    return " > ".join(parts)
            break

    # "Error in" pattern (some CI outputs)
    for line in lines:
        m = re.search(r"Error in\s+['\"]?(.+?)['\"]?\s*[:$]", line)
        if m:
            return m.group(1).strip()

    # Jest/Vitest FAIL line with " > " embedded in same line (Vitest)
    for line in lines:
        m = re.search(r"\bFAIL\b\s+\S.*", line)
        if m:
            segment = m.group(0)
            if " > " in segment:
                return segment.split(" > ", 1)[1].strip()

    # Jest/prettier: "FAIL tests/unit/foo.js\n<describe> > <test>"
    for i, line in enumerate(lines):
        if re.match(r"^FAIL\s+\S+", line.strip()):
            for j in range(i + 1, min(i + 5, len(lines))):
                nxt = lines[j].strip()
                if nxt and " > " in nxt and not nxt.startswith("at "):
                    return nxt

    return None


def extract_error_type(log_content):
    """Extract the exception class from the log (e.g. AssertionError, TypeError)."""
    clean = strip_ansi(log_content)
    # Priority order: AssertionError first, then others
    for exc in [
        "AssertionError",
        "TypeError",
        "ReferenceError",
        "SyntaxError",
        "Error",
    ]:
        pattern = rf"\b{exc}\b[\s:\[]"
        for line in clean.split("\n"):
            if re.search(pattern, line):
                if "Process completed" not in line and "npm ERR" not in line:
                    return exc
    return None


def extract_file(log_content):
    """Extract the source file from the top of the stack trace or error output."""
    clean = strip_ansi(log_content)
    lines = clean.split("\n")

    # 1. TypeScript: ##[error]src/file(line,col): error TSxxxx
    for line in lines:
        m = re.search(r"##\[error\]((?:src|lib)/[^\s(]+)\(\d+,\d+\):", line)
        if m:
            return m.group(1)

    # 2. Build error: src/file.js:line:col: ERROR: (guard against node_modules)
    for line in lines:
        if "node_modules" in line:
            continue
        m = re.search(r"((?:src|lib)/[^\s:]+\.\w+):\d+[,:]?\d*:?\s*(error|ERROR)", line, re.IGNORECASE)
        if m:
            return m.group(1)

    # 3a. ESLint ✘ rule block: indented "src/file.js:line:col" (no trailing error keyword)
    for line in lines:
        stripped = line.strip()
        m = re.match(r"^((?:src|lib)/[\w/.-]+\.\w+):\d+:\d+$", stripped)
        if m:
            return m.group(1)

    # 3b. ESLint ##[error] block: file path on the line BEFORE "##[error]  line:col  error"
    for i, line in enumerate(lines):
        if re.search(r"##\[error\]\s+\d+:\d+\s+error\s+", line):
            for j in range(max(0, i - 6), i):
                candidate = lines[j].strip()
                # Could be /home/runner/work/.../src/file.js
                rel = project_relative(candidate)
                if rel and re.search(r"\.(js|ts|mjs|cjs)$", rel):
                    return rel
                # Or directly "src/file.js"
                if re.match(r"^(?:src|lib)/[\w/.-]+\.\w+$", candidate):
                    return candidate
            break

    # 3c. ESLint plain line:col format (no ##[error] prefix) — file on preceding line
    for i, line in enumerate(lines):
        if re.search(r"^\s+\d+:\d+\s+error\s+", line):
            for j in range(max(0, i - 3), i):
                candidate = lines[j].strip()
                if re.match(r"^/", candidate):
                    rel = project_relative(candidate)
                    if rel and re.search(r"\.(js|ts|mjs|cjs)$", rel):
                        return rel
            break

    # 4. Absolute path from CI output: /home/runner/work/repo/repo/src/file.js
    for line in lines:
        m = re.search(r"/runner/work/[^/]+/[^/]+/((?:src|lib|test)[^\s:]+\.\w+)", line)
        if m:
            rel = m.group(1)
            if not any(skip in rel for skip in ["node_modules", "_temp", "runner_temp"]):
                return rel

    # 5. Stack trace lines: "    at Func (/abs/path.js:line:col)"
    for line in lines:
        # file:/// format
        m = re.search(r"file:///(.+?):(\d+):\d+", line)
        if m:
            rel = project_relative(m.group(1))
            if rel:
                return rel
        # Standard "at Func (/abs/path:line:col)"
        m = re.search(r"at\s+(?:async\s+)?\S+\s+\((/[^)]+):(\d+):\d+\)", line)
        if m:
            rel = project_relative(m.group(1))
            if rel:
                return rel
        # Relative path in parens
        m = re.search(r"at\s+\S+\s+\(((?:lib|src|test)[^)]+):(\d+):\d+\)", line)
        if m:
            rel = project_relative(m.group(1))
            if rel:
                return rel or m.group(1)

    # 6. "Exception during run:" — take first useful "at" line
    exc_start = clean.find("Exception during run:")
    if exc_start != -1:
        for line in clean[exc_start:].split("\n")[1:]:
            m = re.search(r"at\s+\S+\s+\((.+?):(\d+):\d+\)", line)
            if m:
                rel = project_relative(m.group(1))
                if rel:
                    return rel
            # Minified format: "at func (lib\file.js:1:nnn)"
            m = re.search(r"at\s+\S+\s+\((.+?\.js):", line)
            if m:
                return m.group(1).replace("\\", "/")

    # 7. Prettier/lint format warning
    for line in lines:
        m = re.search(r"\[warn\]\s+((?:src|lib)/\S+)", line)
        if m:
            return m.group(1)
        m = re.search(r"\[warn\]\s+(lib/\S+)", line)
        if m:
            return m.group(1)

    # 8. Autofix CI: staged file path shown in git output
    for line in lines:
        stripped = line.strip()
        m = re.match(r"^((?:src|lib)/[\w/.-]+\.(js|ts|mjs|cjs))$", stripped)
        if m:
            return m.group(1)

    return None


def load_index():
    """Load index.json to get split and ordering."""
    index_path = os.path.join(CASES_DIR, "index.json")
    with open(index_path) as f:
        return json.load(f)


def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    index = load_index()
    index_order = {item["id"]: i for i, item in enumerate(index)}

    included = []
    excluded = []

    for entry in sorted(os.listdir(CASES_DIR)):
        case_dir = os.path.join(CASES_DIR, entry)
        if not os.path.isdir(case_dir):
            continue

        meta_path = os.path.join(case_dir, "meta.json")
        diff_path = os.path.join(case_dir, "fix_diff.json")
        log_path = os.path.join(case_dir, "failure_log.txt")

        if not all(os.path.exists(p) for p in [meta_path, diff_path, log_path]):
            continue

        with open(meta_path) as f:
            meta = json.load(f)
        with open(diff_path) as f:
            diff = json.load(f)
        with open(log_path) as f:
            log_content = f.read()

        case_id = meta["id"]
        split = meta.get("split", "training")
        fix_file = meta.get("fix_file", "")
        diff_files = diff.get("files", [])

        # --- Exclusion 1: fix_file is a test file ---
        if is_test_file(fix_file):
            reason = "fix_file is a test file"
            excluded.append({"id": case_id, "split": split, "excluded": True, "reason": reason})
            print(f"  EXCLUDE {case_id}: {reason}")
            continue

        # --- Exclusion 2: multiple production files in fix_diff ---
        prod_count = count_production_files(diff_files)
        if prod_count > 1:
            reason = f"fix_diff has {prod_count} production files"
            excluded.append({"id": case_id, "split": split, "excluded": True, "reason": reason})
            print(f"  EXCLUDE {case_id}: {reason} ({[f for f in diff_files if not is_test_file(f) and not is_lock_file(f)]})")
            continue

        # --- Exclusion 3: no identifiable stack trace ---
        if not has_identifiable_error(log_content):
            reason = "no identifiable stack trace"
            excluded.append({"id": case_id, "split": split, "excluded": True, "reason": reason})
            print(f"  EXCLUDE {case_id}: {reason}")
            continue

        # --- Extract fields ---
        failing_test = extract_failing_test(log_content)
        error_type = extract_error_type(log_content)
        file_ref = extract_file(log_content)

        record = {
            "id": case_id,
            "split": split,
            "failing_test": failing_test,
            "error_type": error_type,
            "file": file_ref,
            "fix_file": fix_file,
        }
        included.append(record)
        print(
            f"  INCLUDE {case_id} [{split}]"
            f" test={repr(failing_test)[:50]}"
            f" err={error_type}"
            f" file={file_ref}"
        )

    # Sort: training first (preserving index order), then held-out
    def sort_key(r):
        split_rank = 0 if r["split"] == "training" else 1
        return (split_rank, index_order.get(r["id"], 9999))

    included.sort(key=sort_key)

    # Write ground_truth.json
    with open(os.path.join(OUTPUT_DIR, "ground_truth.json"), "w") as f:
        json.dump(included, f, indent=2)

    # Write summary.json
    reasons = {}
    for c in excluded:
        reasons[c["reason"]] = reasons.get(c["reason"], 0) + 1

    summary = {
        "total": len(included) + len(excluded),
        "included": len(included),
        "excluded": len(excluded),
        "excluded_by_reason": reasons,
    }
    with open(os.path.join(OUTPUT_DIR, "summary.json"), "w") as f:
        json.dump(summary, f, indent=2)

    print(f"\nIncluded: {len(included)}  Excluded: {len(excluded)}")
    for r, n in reasons.items():
        print(f"  {r}: {n}")


if __name__ == "__main__":
    main()
