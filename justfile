# token-usage

test:
    npx tsx --test tests/*.test.ts

build:
    esbuild src/cli.ts --bundle --platform=node --format=esm --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'

deploy: build
    npm publish

release bump="patch":
    src/scripts/release.sh {{bump}}
