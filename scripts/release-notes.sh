#!/usr/bin/env bash
set -euo pipefail

# Generate release notes into dist/release-notes.md
# Called by: just release-notes [tag]
# If tag is omitted, uses the latest tag on HEAD.
#
# Minor/major releases (vX.Y.0) aggregate all commits back to the previous
# X.Y.0 tag, giving a rolled-up summary of the whole patch series. Patch
# releases show only commits since the immediately previous tag.

TAG="${1:-$(git describe --tags --abbrev=0 HEAD 2>/dev/null || true)}"
if [ -z "$TAG" ]; then
  echo "ERROR: No tag found. Pass a tag or ensure HEAD is tagged."
  exit 1
fi

VERSION="${TAG#v}"
PATCH="${VERSION##*.}"

if [ "$PATCH" = "0" ]; then
  # Minor/major release — walk back to the previous *.0 tag
  PREV=$(git tag -l 'v*' | grep '\.0$' | sort -V | grep -B1 "^${TAG}$" | head -1)
  if [ "$PREV" = "$TAG" ] || [ -z "$PREV" ]; then
    PREV=$(git tag -l 'v*' | sort -V | head -1)
  fi
else
  # Patch release — previous tag by version sort
  PREV=$(git tag -l 'v*' | sort -V | grep -B1 "^${TAG}$" | head -1)
  if [ "$PREV" = "$TAG" ] || [ -z "$PREV" ]; then
    PREV=$(git tag -l 'v*' | sort -V | head -1)
  fi
fi

HEADER="Changes since ${PREV}"

REMOTE_URL=$(git remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|git@github.com:|https://github.com/|')
if [[ "$REMOTE_URL" == https://github.com/* ]]; then
  REPO="${REMOTE_URL#https://github.com/}"
  COMMITS=$(git log --oneline "${PREV}..${TAG}" | sed "s|^\([0-9a-f]\{7,\}\)|[\1](https://github.com/${REPO}/commit/\1)|" | sed 's/^/- /')
else
  COMMITS=$(git log --oneline "${PREV}..${TAG}" | sed 's/^/- /')
fi

mkdir -p dist
printf 'tu release %s\n\n## %s\n\n%s\n' "$VERSION" "$HEADER" "$COMMITS" > dist/release-notes.md
echo "Generated dist/release-notes.md (${PREV}..${TAG})"
