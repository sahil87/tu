#!/usr/bin/env bash
set -euo pipefail

# Generate the Go Homebrew formula into dist/tu.rb — release.yml pushes it to
# sahil87/homebrew-tap (plan row X1).
# Called by: just go-formula [tag]
# Requires: just go-package (tu-go archives must exist in dist/)

TAG="${1:-$(git describe --tags --abbrev=0 HEAD 2>/dev/null || true)}"
if [ -z "$TAG" ]; then
  echo "ERROR: No tag found. Pass a tag or ensure HEAD is tagged." >&2
  exit 1
fi

VERSION="${TAG#v}"

for platform in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
  archive="dist/tu-go-${platform}.tar.gz"
  if [ ! -f "$archive" ]; then
    echo "ERROR: Missing $archive — run 'just go-package' first." >&2
    exit 1
  fi
done

sha_darwin_arm64=$(shasum -a 256 dist/tu-go-darwin-arm64.tar.gz | cut -d' ' -f1)
sha_darwin_amd64=$(shasum -a 256 dist/tu-go-darwin-amd64.tar.gz | cut -d' ' -f1)
sha_linux_arm64=$(shasum -a 256 dist/tu-go-linux-arm64.tar.gz | cut -d' ' -f1)
sha_linux_amd64=$(shasum -a 256 dist/tu-go-linux-amd64.tar.gz | cut -d' ' -f1)

sed \
  -e "s/VERSION_PLACEHOLDER/${VERSION}/" \
  -e "s/SHA_DARWIN_ARM64/${sha_darwin_arm64}/" \
  -e "s/SHA_DARWIN_AMD64/${sha_darwin_amd64}/" \
  -e "s/SHA_LINUX_ARM64/${sha_linux_arm64}/" \
  -e "s/SHA_LINUX_AMD64/${sha_linux_amd64}/" \
  .github/formula-template.rb > dist/tu.rb

echo "Generated dist/tu.rb (v${VERSION})"
