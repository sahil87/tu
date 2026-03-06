#!/usr/bin/env bash
set -euo pipefail

bump="${1:?Usage: release.sh <patch|minor|major>}"
tap="../homebrew-tap"

# 1. Bump version in package.json (no git tag — we control that)
version=$(npm version "$bump" --no-git-tag-version | tr -d 'v')
echo "Releasing v$version"

# 2. Commit the version bump, tag, and push
git add package.json
git commit -m "v$version"
git tag "v$version"
git push && git push --tags

# 3. Get the SHA of the tarball
sha=$(curl -sL "https://github.com/wvrdz/token-usage/archive/refs/tags/v${version}.tar.gz" | shasum -a 256 | awk '{print $1}')
echo "SHA: $sha"

# 4. Update formula
sed -i '' \
    -e "s|archive/refs/tags/v.*\.tar\.gz|archive/refs/tags/v${version}.tar.gz|" \
    -e "s|sha256 \".*\"|sha256 \"$sha\"|" \
    "$tap/Formula/tu.rb"

# 5. Push homebrew-tap
cd "$tap"
git add Formula/tu.rb
git commit -m "tu $version"
git push

echo "Done — v$version released"
