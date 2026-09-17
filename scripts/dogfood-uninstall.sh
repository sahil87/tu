#!/usr/bin/env bash
set -euo pipefail

# Remove the Go dogfood build so `tu` resolves to the brew binary again
# (plan row R2 step 5, run before X1). Idempotent: ~/.local/bin/tu is removed
# only when it is a symlink into ~/.local/lib/tu-go/ (a hand-installed binary
# is never clobbered).
# Called by: just dogfood-uninstall

LIB_DIR="$HOME/.local/lib/tu-go"
BIN_LINK="$HOME/.local/bin/tu"

if [ -L "$BIN_LINK" ]; then
  target=$(readlink "$BIN_LINK")
  case "$target" in
    "$LIB_DIR"/*)
      rm -f "$BIN_LINK"
      ;;
    *)
      echo "leaving ~/.local/bin/tu in place (not a dogfood symlink)"
      ;;
  esac
elif [ -e "$BIN_LINK" ]; then
  echo "leaving ~/.local/bin/tu in place (not a dogfood symlink)"
fi

rm -rf "$LIB_DIR"

first=$(which -a tu 2>/dev/null | head -1 || true)
if [ -n "$first" ]; then
  echo "Removed dogfood build; tu now resolves to: $first"
else
  echo "Removed dogfood build; no tu on PATH"
fi
