#!/usr/bin/env bash
set -euo pipefail

VERSION=$(node -p 'require("./package.json").version')

npx esbuild src/node/core/cli.ts \
  --bundle --platform=node --format=esm \
  --outfile=dist/tu.mjs \
  --banner:js='#!/usr/bin/env node' \
  --define:__PKG_VERSION__="\"$VERSION\""

rm -rf dist/vendor
mkdir -p dist/vendor/ccusage dist/vendor/ccusage-codex dist/vendor/ccusage-opencode
cp node_modules/ccusage/dist/*.js dist/vendor/ccusage/
cp node_modules/@ccusage/codex/dist/*.js dist/vendor/ccusage-codex/
cp node_modules/@ccusage/opencode/dist/*.js dist/vendor/ccusage-opencode/
