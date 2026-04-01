# tu

test:
    npx tsx --test 'src/node/**/__tests__/*.test.ts'

run *ARGS:
    npx tsx src/node/core/cli.ts {{ARGS}}

build:
    esbuild src/node/core/cli.ts --bundle --platform=node --format=esm --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'
    rm -rf dist/vendor
    mkdir -p dist/vendor/ccusage dist/vendor/ccusage-codex dist/vendor/ccusage-opencode
    cp node_modules/ccusage/dist/*.js dist/vendor/ccusage/
    cp node_modules/@ccusage/codex/dist/*.js dist/vendor/ccusage-codex/
    cp node_modules/@ccusage/opencode/dist/*.js dist/vendor/ccusage-opencode/

deploy: build
    npm publish

release bump="patch":
    scripts/release.sh {{bump}}
