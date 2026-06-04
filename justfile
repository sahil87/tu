# tu
setup:
    npm install

# Run the test suite. Delegates to `npm test` so `just test` matches exactly
# what CI's ci-gate enforces (the prior inline `**` glob did not resolve under
# Node 20 — see package.json's find-based runner).
test:
    npm test

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
