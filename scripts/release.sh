#!/usr/bin/env bash
set -euo pipefail

# scripts/release.sh — Bump version, commit, tag, and push.
#
# CI takes over from the tag push to generate release notes, create the
# GitHub Release, and update the Homebrew tap formula.
# See .github/workflows/release.yml.
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
    exit 1
  fi
  exit 0
fi

# ── Pre-flight ───────────────────────────────────────────────────────

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
tag="v$version"

echo "Bumping version → $version ($bump_type)"

# ── Commit, tag, and push ───────────────────────────────────────────

git -C "$repo_root" add package.json package-lock.json
git -C "$repo_root" commit -m "$tag"
git -C "$repo_root" tag "$tag"
git -C "$repo_root" push origin HEAD:"$branch" "$tag"

echo ""
echo "Release tagged: $tag"
echo "  Tag:     $tag"
echo "  Version: $version"
echo "  Branch:  $branch"
echo ""
echo "CI will generate release notes, create the GitHub Release, and update the Homebrew tap."
