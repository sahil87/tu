# tu
setup:
    npm install

test:
    npx tsx --test 'src/node/**/__tests__/*.test.ts'

run *ARGS:
    npx tsx src/node/core/cli.ts {{ARGS}}

build:
    scripts/build.sh

# Bump version, commit, tag, and push (CI handles the rest)
release bump="patch":
    scripts/release.sh {{bump}}

# Generate release notes for the current tag into dist/release-notes.md
release-notes tag="":
    scripts/release-notes.sh {{tag}}
