#!/usr/bin/env bash
set -euo pipefail

bump="${1:?Usage: release.sh <patch|minor|major>}"
tap="../homebrew-tap"

# 1. Bump version in package.json (no git tag — we control that)
version=$(npm version "$bump" --no-git-tag-version | tr -d 'v')
echo "Releasing v$version"

# 2. Commit the version bump, tag, and push
git add package.json package-lock.json
git commit -m "v$version"
git tag "v$version"
git push && git push --tags

# 3. Create a GitHub Release
gh release create "v$version" --title "v$version" --generate-notes

# 4. Update formula tag
sed -i '' \
    -e "s|tag: \"v.*\"|tag: \"v${version}\"|" \
    "$tap/Formula/tu.rb"

# 5. Push homebrew-tap
cd "$tap"
git add Formula/tu.rb
git commit -m "tu $version"
git push

echo "Done — v$version released"
