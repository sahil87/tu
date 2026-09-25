# tu

# Create and push the next version tag; CI takes it from there (the tag is the
# version anchor — no file edits, no commit).
release bump="patch":
    scripts/release.sh {{bump}}

# Generate release notes for the current tag into dist/release-notes.md
release-notes tag="":
    scripts/release-notes.sh {{tag}}

# ── The Go module (src/go/) is the whole tool: what the Homebrew formula ships,
# what CI gates on, what the differential harness checks against the committed
# golden corpus (harness/golden/). Plan: fab/plans/sahil/26-09-15-go-port.md.

# Version stamp for the Go binary: the git tag is the single version anchor
# (release.sh only tags; the tag carries the v). The golden corpus's $VERSION
# normalization keeps the differential harness byte-matching `--version`
# across tags.
go_version := `git describe --tags --always 2>/dev/null || echo dev`

# Build-time drift guard for the embedded skill bundle — fails before `go build`
# when the committed copy drifts from the canonical docs/site/skill.md. Shared
# by go-build and go-build-target.
_go-skill-guard:
    cmp -s docs/site/skill.md src/go/internal/toolkit/skill.md || { echo "error: src/go/internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh" >&2; exit 1; }

# Build the Go binary into bin/tu (gitignored).
go-build:
    just _go-skill-guard
    mkdir -p bin
    cd src/go && go build -ldflags "-X main.version={{go_version}}" -o ../../bin/tu ./cmd/tu
    cd src/go && go build -o ../../bin/turepair ./cmd/turepair

# Cross-compile bin for one target into dist/bin/tu-<os>-<arch> (release artifact
# staging; bin/tu stays the dev binary). Only cmd/tu is cross-compiled — turepair
# is a maintainer tool, not shipped. CGO_ENABLED=0 for static binaries.
go-build-target os arch:
    just _go-skill-guard
    mkdir -p dist/bin
    cd src/go && CGO_ENABLED=0 GOOS={{os}} GOARCH={{arch}} go build -ldflags "-X main.version={{go_version}}" -o ../../dist/bin/tu-{{os}}-{{arch}} ./cmd/tu

# The four release targets (Homebrew's matrix: darwin/linux x arm64/amd64).
go-build-all:
    just go-build-target darwin arm64
    just go-build-target darwin amd64
    just go-build-target linux arm64
    just go-build-target linux amd64

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

# Byte-diff bin/tu against the committed golden corpus (harness/golden/) over
# harness/matrix.json (plan row Z1).
# Gate: exit 1 on an unexpected red case, a timeout, an unconfirmed-fixture
# replay, or a stale entry in harness/expected-diffs.json.
go-diff *ARGS: go-build harness-build
    bin/harness/tudiff run {{ARGS}}

# Real-git parity: the sync/repair sequence against temp bare repos, compared
# against harness/golden/live/ and reported under bin/harness/report-live/.
# Same gate rule as go-diff: exit 1 on an unexpected red step, a timeout, an
# unconfirmed-fixture replay, or a stale expected-diffs entry.
go-live *ARGS: go-build harness-build
    bin/harness/tudiff live {{ARGS}}

# Package the four tu-go-<os>-<arch>.tar.gz release archives (+ SHA256SUMS, host
# smoke test) into dist/. Requires go-build-all; fetches the pinned ccusage
# tarballs from the package registry by curl (no package manager in the fetch path).
go-package:
    scripts/package-go.sh

# Generate the Go Homebrew formula into dist/tu.rb (echoed in the release log;
# release.yml pushes it to the tap as the last release step — plan row X1).
go-formula tag="":
    scripts/go-formula.sh {{tag}}

# Local end-to-end release pipeline: everything the release job runs minus the
# uploads (fab-kit's `dist` recipe shape).
go-dist: go-build-all
    just go-package
    just go-formula

# Install the Go dogfood build from a release's tu-go-* asset into ~/.local
# (plan rows R1/R2). Default: latest release.
dogfood-install tag="":
    scripts/dogfood-install.sh {{tag}}

# Remove the dogfood build so `tu` resolves to the brew binary again.
dogfood-uninstall:
    scripts/dogfood-uninstall.sh
