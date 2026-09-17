#!/usr/bin/env bash
set -euo pipefail

# Install the Go dogfood build from a release's tu-go-* asset into ~/.local
# (plan rows R1/R2): download the host's tarball + SHA256SUMS via gh, verify
# the sha, extract to ~/.local/lib/tu-go/, symlink ~/.local/bin/tu, and report
# the PATH order so it shadows the brew tu.
# Called by: just dogfood-install [tag]  (default: latest published release)
# Requires: gh (authenticated), tar, shasum

REPO="sahil87/tu"
ASSET_PREFIX="tu-go"
LIB_DIR="$HOME/.local/lib/tu-go"
BIN_LINK="$HOME/.local/bin/tu"

# 1. Resolve the tag: $1, else the latest published release.
tag="${1:-}"
if [ -z "$tag" ]; then
  tag=$(gh release view --repo "$REPO" --json tagName -q .tagName)
fi
echo "Release: $tag"

# 2. Host detection (same mapping as the release matrix).
case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "error: unsupported host OS: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "error: unsupported host arch: $(uname -m)" >&2; exit 1 ;;
esac
asset="${ASSET_PREFIX}-${os}-${arch}.tar.gz"
sums="${ASSET_PREFIX}-SHA256SUMS"

# 3. Download the two assets.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
gh release download "$tag" --repo "$REPO" --pattern "$asset" --pattern "$sums" --dir "$tmp" || true
if [ ! -f "$tmp/$asset" ]; then
  echo "error: release $tag has no $asset asset — Go assets exist only for releases cut after plan row R1 landed" >&2
  exit 1
fi
if [ ! -f "$tmp/$sums" ]; then
  echo "error: release $tag has no $sums asset — Go assets exist only for releases cut after plan row R1 landed" >&2
  exit 1
fi

# 4. Verify the sha256 against the sums file; nothing is installed on mismatch.
expected=$(awk -v f="$asset" '$2 == f {print $1}' "$tmp/$sums")
got=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)
if [ -z "$expected" ] || [ "$expected" != "$got" ]; then
  echo "error: sha256 mismatch for $asset (expected $expected, got $got)" >&2
  exit 1
fi

# 5. Refuse to clobber a hand-installed ~/.local/bin/tu; only a symlink into
# the dogfood lib dir (ours from a previous run) may be replaced.
if [ -e "$BIN_LINK" ] || [ -L "$BIN_LINK" ]; then
  target=$(readlink "$BIN_LINK" 2>/dev/null || true)
  case "$target" in
    "$LIB_DIR"/*) ;;
    *)
      echo "error: ~/.local/bin/tu exists and is not a dogfood symlink — remove it first" >&2
      exit 1
      ;;
  esac
fi

# 6. Install: the vendor tree must sit beside the real binary, so the binary
# lives in the lib dir and ~/.local/bin holds only the symlink the resolver
# follows (ccusage.ResolveBinary resolves symlinks, then looks beside the real
# file).
rm -rf "$LIB_DIR"
mkdir -p "$LIB_DIR"
tar xzf "$tmp/$asset" -C "$LIB_DIR"
mkdir -p "$HOME/.local/bin"
ln -sfn "$LIB_DIR/tu" "$BIN_LINK"

# 7. Report.
echo "Installed: ~/.local/bin/tu -> ~/.local/lib/tu-go/tu ($("$LIB_DIR/tu" --version))"
echo "PATH order for tu:"
i=0
while IFS= read -r p; do
  i=$((i + 1))
  case "$p" in
    "$BIN_LINK") note="<- dogfood (Go)" ;;
    */.linuxbrew/* | */homebrew/* | */Cellar/*) note="<- brew (Node)" ;;
    *) note="<- other" ;;
  esac
  printf '  %d. %s  %s\n' "$i" "$p" "$note"
done < <(which -a tu 2>/dev/null || true)

first=$(which -a tu 2>/dev/null | head -1 || true)
if [ "$first" = "$BIN_LINK" ]; then
  echo "OK: the dogfood build shadows the brew tu. Run \`hash -r\` (or open a new shell) if \`tu\` still resolves to brew."
else
  echo "WARNING: $first wins on PATH — the brew tu still runs. Prepend ~/.local/bin to PATH (e.g. in ~/.zshrc) and re-run."
fi
echo "Note: \`tu update\` on this binary prints the not-installed-via-Homebrew message (the /Cellar/tu/ gate) — refresh with \`just dogfood-install\` after each fix release (plan row R2 step 3)."
