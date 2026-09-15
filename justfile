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

# ── Go successor (src/go/) — built and tested in CI, NOT shipped until cutover ──
# Constitution v1.2.0 § Go Transition; plan: fab/plans/sahil/26-09-15-go-port.md.

# Version stamp for the Go binary. package.json is the single version anchor
# during the transition (release.sh bumps it; the v* tag is derived from it),
# and the TS binary prints exactly this value — so the differential harness
# (plan row P4) byte-matches `--version` across both implementations. Z1
# switches this to `git describe` when package.json goes away.
go_version := `node -p 'require("./package.json").version'`

# Build the Go binary into bin/tu (gitignored; not dist/, which is the shipped Node artifact).
go-build:
    mkdir -p bin
    cd src/go && go build -ldflags "-X main.version=v{{go_version}}" -o ../../bin/tu ./cmd/tu

# Run the Go test suite under src/go/.
go-test:
    cd src/go && go test ./... -count=1

# gofmt + go vet over src/go/ — the same two checks the sibling Go tools gate CI on.
go-lint:
    #!/usr/bin/env bash
    set -euo pipefail
    cd src/go
    unformatted="$(gofmt -l .)"
    if [ -n "$unformatted" ]; then
        echo "The following files are not gofmt-clean:" >&2
        echo "$unformatted" >&2
        echo "Run: (cd src/go && gofmt -w .)" >&2
        exit 1
    fi
    go vet ./...

# Build the harness binaries into bin/harness/ (gitignored via bin/). The fakes are
# named for the tools they impersonate so P4 can prepend bin/harness to PATH.
harness-build:
    mkdir -p bin/harness
    cd src/go && go build -o ../../bin/harness/tudiff ./cmd/tudiff
    cd src/go && go build -o ../../bin/harness/ccusage ./cmd/fakeccusage
    cd src/go && go build -o ../../bin/harness/git ./cmd/fakegit

# Record real ccusage output for this machine into harness/fixtures/<alias>/ (default alias: hostname).
harness-capture *ARGS: harness-build
    bin/harness/tudiff capture {{ARGS}}
