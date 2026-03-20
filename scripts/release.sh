#!/usr/bin/env bash
set -euo pipefail

# scripts/release.sh — Bump version, commit, tag, push, create GitHub
# Release, and update the Homebrew tap formula.
#
# Usage: release.sh <patch|minor|major>
#   patch — 0.1.0 → 0.1.1
#   minor — 0.1.0 → 0.2.0
#   major — 0.1.0 → 1.0.0

usage() {
  echo "Usage: release.sh <patch|minor|major>"
  echo ""
  echo "  patch — bump patch version (e.g. 0.1.0 → 0.1.1)"
  echo "  minor — bump minor version (e.g. 0.1.0 → 0.2.0)"
  echo "  major — bump major version (e.g. 0.1.0 → 1.0.0)"
}

repo_root="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
tap="$repo_root/../homebrew-tap"

# ── Parse arguments ──────────────────────────────────────────────────

bump_type=""

for arg in "$@"; do
  case "$arg" in
    patch|minor|major)
      if [ -n "$bump_type" ]; then
        echo "ERROR: Multiple bump types specified: '$bump_type' and '$arg'."
        echo ""
        usage
        exit 1
      fi
      bump_type="$arg"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "ERROR: Unknown argument '$arg'. Use: patch, minor, or major."
      echo ""
      usage
      exit 1
      ;;
  esac
done

if [ -z "$bump_type" ]; then
  usage
  if [ $# -gt 0 ]; then
    exit 1  # Had flags but no bump type — that's an error
  fi
  exit 0
fi

# ── Pre-flight ───────────────────────────────────────────────────────

# Check clean working tree
if [ -n "$(git -C "$repo_root" status --porcelain)" ]; then
  echo "ERROR: Working tree not clean. Commit or stash changes first."
  exit 1
fi

branch=$(git -C "$repo_root" branch --show-current)
if [ -z "$branch" ]; then
  echo "ERROR: Not on a branch (detached HEAD). Check out a branch before releasing."
  exit 1
fi

# ── Bump version ─────────────────────────────────────────────────────

version=$(cd "$repo_root" && npm version "$bump_type" --no-git-tag-version | tr -d 'v')
echo "Releasing v$version"

# ── Commit, tag, and push ───────────────────────────────────────────

git -C "$repo_root" add package.json package-lock.json
git -C "$repo_root" commit -m "v$version"
git -C "$repo_root" tag "v$version"
git -C "$repo_root" push origin HEAD:"$branch" --tags

# ── GitHub Release ───────────────────────────────────────────────────

gh release create "v$version" --title "v$version" --generate-notes

# ── Update Homebrew tap ──────────────────────────────────────────────

if [ ! -d "$tap" ]; then
  echo "Warning: Homebrew tap not found at $tap — skipping formula update."
else
  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "s|tag: \"v.*\"|tag: \"v${version}\"|" "$tap/Formula/tu.rb"
  else
    sed -i "s|tag: \"v.*\"|tag: \"v${version}\"|" "$tap/Formula/tu.rb"
  fi

  cd "$tap"
  git add Formula/tu.rb
  git commit -m "tu $version"
  git push
fi

echo ""
echo "Done — v$version released"
