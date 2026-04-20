package spec

import _ "embed"

// Content is the embedded JUC specification, written to ~/.claude/skills/juc-spec.md by juc install.
//
//go:embed SPEC.md
var Content []byte
