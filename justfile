# tu

test:
    npx tsx --test 'src/node/**/__tests__/*.test.ts'

run *ARGS:
    npx tsx src/node/core/cli.ts {{ARGS}}

build:
    esbuild src/node/core/cli.ts --bundle --platform=node --format=esm --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'

deploy: build
    npm publish

release bump="patch":
    scripts/release.sh {{bump}}
