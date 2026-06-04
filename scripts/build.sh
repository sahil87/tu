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
mkdir -p dist/vendor/ccusage dist/vendor/ccusage-codex dist/vendor/ccusage-opencode
cp node_modules/ccusage/dist/*.js dist/vendor/ccusage/
cp node_modules/@ccusage/codex/dist/*.js dist/vendor/ccusage-codex/
cp node_modules/@ccusage/opencode/dist/*.js dist/vendor/ccusage-opencode/
