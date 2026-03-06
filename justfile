# token-usage

test:
    npx tsx --test 'src/node/**/__tests__/*.test.ts'

build:
    esbuild src/node/core/cli.ts --bundle --platform=node --format=esm --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'

deploy: build
    npm publish

release bump="patch":
    src/node/scripts/release.sh {{bump}}
