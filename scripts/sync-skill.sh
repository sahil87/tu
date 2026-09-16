#!/usr/bin/env bash
# Copy the canonical docs/site/skill.md into src/go/internal/toolkit/ so it can
# be embedded via //go:embed. The Go module root is src/go/ and docs/site/ sits
# above it, so embed cannot reach the canonical file directly — this copy step
# bridges the gap. The copy is committed so a clean `go build ./...` (which does
# not run this script) compiles; the drift-guard test in
# src/go/internal/toolkit/skill_test.go and the cmp guard in `just go-build`
# keep it byte-honest against docs/site/skill.md.
set -euo pipefail

# Run from the repo root regardless of caller CWD.
cd "$(dirname "$0")/.."

SRC="docs/site/skill.md"
DEST="src/go/internal/toolkit/skill.md"

mkdir -p "$(dirname "$DEST")"
cp -f "$SRC" "$DEST"
echo "synced skill bundle: ${DEST}"
