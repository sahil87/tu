#!/usr/bin/env bash
set -euo pipefail

# Embed package metadata at build time. The bottle ships only dist/ (no
# package.json at runtime), so `tu help-dump` reads name/description/version
# from these defines. JSON.stringify yields a correctly-quoted JS string
# literal (handles spaces/quotes in description); each define is a single
# pre-quoted shell word, so no extra escaping is needed.
VERSION_DEF=$(node -p 'JSON.stringify(require("./package.json").version)')
NAME_DEF=$(node -p 'JSON.stringify(require("./package.json").name)')
DESC_DEF=$(node -p 'JSON.stringify(require("./package.json").description ?? "")')

npx esbuild src/node/core/cli.ts \
  --bundle --platform=node --format=esm \
  --outfile=dist/tu.mjs \
  --banner:js='#!/usr/bin/env node' \
  --define:__PKG_VERSION__="$VERSION_DEF" \
  --define:__PKG_NAME__="$NAME_DEF" \
  --define:__PKG_DESCRIPTION__="$DESC_DEF"

rm -rf dist/vendor
# ccusage@20 ships no JS implementation — the native Rust binary lives in a
# per-platform optionalDependency (@ccusage/ccusage-{platform}-{arch}) that npm
# installs for the host at `npm install` time. Vendor the host's binary directly.
# Fail loud: this is build-time (not a runtime data source), so an absent binary
# is a hard error, not graceful degradation.
PLATFORM_PKG=$(node -p '`@ccusage/ccusage-${process.platform}-${process.arch}`')
BIN_SRC="node_modules/${PLATFORM_PKG}/bin/ccusage"
if [ ! -f "$BIN_SRC" ]; then
  echo "error: ${PLATFORM_PKG} not installed — unsupported platform or npm install skipped optional deps" >&2
  exit 1
fi
mkdir -p dist/vendor/ccusage/bin
cp "$BIN_SRC" dist/vendor/ccusage/bin/ccusage
chmod 0755 dist/vendor/ccusage/bin/ccusage
