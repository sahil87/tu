# tu
setup:
    npm install

test:
    npx tsx --test 'src/node/**/__tests__/*.test.ts'

run *ARGS:
    npx tsx src/node/core/cli.ts {{ARGS}}

build:
    scripts/build.sh

deploy: build
    npm publish

release bump="patch":
    scripts/release.sh {{bump}}
