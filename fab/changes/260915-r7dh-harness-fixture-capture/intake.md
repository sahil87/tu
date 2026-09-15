# Intake: Harness Fixture Capture

**Change**: 260915-r7dh-harness-fixture-capture
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation, handed over as row **P3a** of the Go-port plan (`fab/plans/sahil/26-09-15-go-port.md`). Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row P3a. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal.
>
> Row P3a (harness-fixture-capture, split from P3): D7 fixture capture. Build src/go/cmd/tudiff with a capture mode: tudiff capture wraps the real vendored ccusage binary, records its JSON output per (source, period, args) into harness/fixtures/<machine-alias>/, and redacts project paths. Also build a fake ccusage replayer and a fake git for later use by the harness. Run the capture on this machine (dev-ws-sahil02 equivalent — whichever agents data this machine has, which per the plan is Claude data only) and commit those fixtures. For the five other sources (codex, opencode, gemini, copilot, kimi), generate schema-derived placeholder fixtures from the ccusage v20 JSON shape and mark each unconfirmed: true in a fixture manifest so the harness can report them separately later. Depends on P2 (go scaffold, already merged, so src/go/go.mod and CI exist).

Context read before writing this intake: the plan's **Decisions** (D1–D13, especially D5, D6, D7, D11) and **Target architecture** sections; constitution v1.2.0 § Go Transition; `fab/project/context.md`; `docs/memory/build/toolchain.md`, `docs/memory/cli/data-pipeline.md`; `docs/specs/usage.md` § ccusage invocation; the P2 intake (`260915-h1iv-go-scaffold`) as the precedent for Go-side conventions; `src/node/core/fetcher.ts` (how tu invokes ccusage), `src/node/sync/sync.ts` and `src/node/core/cli.ts` (which `git` commands tu runs); `justfile`, `.github/workflows/ci.yml`, `scripts/build.sh`. P2 is merged (`src/go/go.mod`, `src/go/cmd/tu/`, `just go-build/go-test/go-lint`, CI lane `go-build-and-test`). This worktree was fast-forwarded to `origin/main` (`9361d78`) before writing.

**Live probe performed on this machine (dev-ws-sahil02, 2026-09-16), which corrects the row's premise.** After `npm ci`, the vendored `ccusage 20.0.19` was run as `ccusage <agent> daily --json` for all six agents:

| ccusage subcommand | tu key | exit | `daily` entries | date range | cost key | notes |
|---|---|---|---|---|---|---|
| `claude` | `cc` | 0 | 9 | 2026-09-08 → 2026-09-16 | `totalCost` | `modelBreakdowns[]`, `modelsUsed[]` |
| `codex` | `codex` | 0 | 17 | 2026-08-05 → 2026-09-13 | `costUSD` | `models{}` map, extra `reasoningOutputTokens` |
| `opencode` | `oc` | 0 | 0 | — | (`totalCost` in `totals`) | empty; `totals.totalCost` is literally `-0.0` |
| `gemini` | `gemini` | 0 | 1 | 2026-08-08 | `totalCost` | same shape as claude |
| `copilot` | `copilot` | 0 | 0 | — | (`totalCost` in `totals`) | empty; `totals.totalCost` is literally `-0.0` |
| `kimi` | `kimi` | 0 | 37 | 2026-08-09 → 2026-09-16 | `totalCost` | same shape as claude; model names like `fireworks-ai/accounts/fireworks/routers/kimi-k3-fast` |

stderr was empty for all six. No string in any output contains `/home/`, `/Users/`, or the capturing `$HOME` (the default `daily --json` carries no project paths; those appear only under `--instances`/`session`, which tu never calls). So **this machine yields real, confirmed fixtures for four sources (claude, codex, gemini, kimi) and real *empty* captures for two (opencode, copilot)**. The plan's D7 note "dev-ws-sahil02 has Claude data only" is stale. The user's own phrasing — "whichever agents data this machine has" — already covers this: capture what is real, generate placeholders only for the sources that returned no data (opencode, copilot). The plan doc is updated accordingly (§ 8 below).

## Why

**Problem.** D6 makes the differential harness the release gate for the Go port: byte-diff `node dist/tu.mjs` against the Go binary over a fixture corpus, with a fake `ccusage` replaying recorded JSON and a fake `git` so sync paths are deterministic and network-free. That corpus does not exist. Without recorded real ccusage output the harness (P4) has nothing to replay, V1's `source/ccusage` adapter has nothing to unit-test against, and every later row (B2 history, B3 multi-mode, B6 sync) would be validated against hand-typed JSON that may not match what ccusage v20 actually emits (the codex `costUSD`/`models` shape and the `-0.0` empty totals above are exactly the kind of detail hand-typed fixtures miss).

**Consequence of not doing it.** P4 cannot start (it depends on P3a). Worse, a harness fed with guessed fixtures gives false confidence: the port could be byte-identical on synthetic data and diverge on the real serializer output the moment it ships.

**Why this shape.** D7 says fixture capture is real work: a capture *mode* that wraps the real binary, so a fixture is always a verbatim recording, never a transcription; redaction so fixtures can be committed; a manifest that separates confirmed captures from schema-derived placeholders so the harness can report them separately (and P3b/G2 can see what is still `unconfirmed`). The replayer and fake git are built now because they are the *consumers* of the fixture layout — defining the on-disk contract without its reader invites drift when P4 lands. Everything is Go under `src/go/` (D5 layout, `_test.go` siblings, zero dependencies as in P2) plus a language-neutral `harness/` corpus, exactly as D6 describes. Nothing here touches any external surface listed under the plan's Goal, `src/node/`, `dist/`, the formula, or the release pipeline.

## What Changes

### 1. Layout

```
src/go/
  cmd/tudiff/main.go            # subcommand dispatcher: capture | placeholder | run (P4 placeholder)
  cmd/fakeccusage/main.go       # replayer; built as bin/harness/ccusage
  cmd/fakegit/main.go           # recording stub; built as bin/harness/git
  internal/harness/
    manifest.go                 # Manifest/Fixture types, path layout, argv key
    redact.go                   # path redaction on raw bytes
    capture.go                  # run the matrix against a real ccusage, write fixtures + manifest
    placeholder.go              # deterministic schema-derived fixture generator
    replay.go                   # argv -> fixture lookup across an ordered list of fixture dirs
    calllog.go                  # JSON-lines invocation log shared by both fakes
    *_test.go                   # siblings (constitution § Go Transition)
    corpus_test.go              # validates every committed manifest + fixture under harness/fixtures/
harness/
  fixtures/
    dev-ws-sahil02/
      manifest.json
      claude/daily.json  codex/daily.json  opencode/daily.json
      gemini/daily.json  copilot/daily.json  kimi/daily.json
    _placeholder/
      manifest.json
      opencode/daily.json  copilot/daily.json
```

The first `src/go/internal/` package lands here. Stdlib only (`flag`, `os/exec`, `encoding/json`, `crypto/sha256`, `regexp`, `path/filepath`); no cobra, no YAML library — the manifest is JSON so the zero-dependency posture from P2 holds and `setup-go`'s `cache: false` stays valid.

### 2. `tudiff capture`

```
tudiff capture [--machine <alias>] [--ccusage <path>] [--out <dir>] [--sources a,b,...] [--periods daily]
```

- `--machine` defaults to `os.Hostname()` (here `dev-ws-sahil02`). The alias is the fixture directory name and is deliberately not redacted — the plan names machines explicitly (D7, P3b).
- `--ccusage` resolution when omitted, first hit wins, run from the repo root (found by walking up to the directory containing `package.json`): `dist/vendor/ccusage/bin/ccusage` (the vendored binary after `npm run build`) → `node_modules/@ccusage/ccusage-<platform>-<arch>/bin/ccusage` (the native binary npm installs; platform/arch use Node's spelling — `runtime.GOOS` maps directly, `GOARCH` `amd64`→`x64`, `arm64`→`arm64`) → `ccusage` on `PATH` → error `tudiff: no ccusage binary found (run npm ci or pass --ccusage)`, exit 1. The `node_modules/.bin/ccusage` JS launcher is not used (it needs `node`; the native binary is what ships).
- `--out` defaults to `harness/fixtures`. `--sources` defaults to the six ccusage subcommands tu uses, in registry order: `claude,codex,opencode,gemini,copilot,kimi`. `--periods` defaults to `daily`, because tu only ever calls `daily` (`docs/specs/usage.md` § 215; `extraArgs` is `[]` at every `fetchHistory`/`fetchTotals` call site in `cli.ts` — `--since`/`--until` are applied client-side). The argument set per cell is exactly `[--json]`.
- Per cell the tool runs `<ccusage> <source> <period> --json` via `exec.CommandContext` with a 120 s timeout, cwd = repo root, environment inherited. It records stdout bytes, stderr bytes, and the exit code. A non-zero exit or timeout is **recorded, not fatal** (a failing source is itself a fixture the harness needs — Constitution II), and the run continues; the summary line lists any non-zero cells.
- stdout is passed through redaction (§ 4) and written **verbatim** (no JSON re-serialization — key order, indentation, and `-0.0` must survive) to `<out>/<alias>/<source>/<period>.json`. Non-empty stderr is written to `<source>/<period>.stderr.txt`; empty stderr writes no file.
- The machine's `manifest.json` is regenerated wholesale on every run (idempotent overwrite of that machine's directory only; other machine directories are untouched).
- Prints one line per cell, e.g. `claude daily --json  exit=0  days=9  2026-09-08..2026-09-16  redactions=0  -> dev-ws-sahil02/claude/daily.json`, and a final summary.

### 3. Manifest (`harness/fixtures/<alias>/manifest.json`)

```json
{
  "schema": 1,
  "machine": "dev-ws-sahil02",
  "captured_at": "2026-09-16T04:55:12Z",
  "ccusage_version": "20.0.19",
  "ccusage_path": "node_modules/@ccusage/ccusage-linux-x64/bin/ccusage",
  "platform": "linux/amd64",
  "timezone": "Asia/Kolkata",
  "fixtures": [
    {
      "source": "claude",
      "period": "daily",
      "args": ["--json"],
      "file": "claude/daily.json",
      "stderr_file": "",
      "exit_code": 0,
      "sha256": "…",
      "days": 9,
      "first_date": "2026-09-08",
      "last_date": "2026-09-16",
      "empty": false,
      "redactions": 0,
      "unconfirmed": false
    }
  ]
}
```

- `source` is the **ccusage subcommand** (`claude`, not tu's `cc`; `opencode`, not `oc`) because the replayer matches ccusage argv. The tu-key mapping is documented in memory, not duplicated in the manifest.
- `ccusage_version` comes from `<ccusage> --version` (prints `ccusage 20.0.19`); `timezone` is the IANA zone ccusage grouped by (`$TZ` if set, else the system zone read via `time.Now().Location()` / `/etc/timezone` fallback), recorded because ccusage buckets dates in the system zone and P4 runs under a fixed `TZ`.
- `days`, `first_date`, `last_date`, `empty` are derived by parsing the recorded stdout as `{daily: [{date}]}`; when stdout is not parseable JSON (a failed source) they are `0`, `""`, `""`, `true` and `exit_code` carries the failure.
- `sha256` is over the committed file bytes; the corpus test (§ 7) verifies it, so a hand-edited fixture without a manifest update fails CI.
- `unconfirmed` is `false` for every real capture, including the real empty opencode/copilot captures — an empty result from a machine that has no OpenCode/Copilot transcripts is a genuine, confirmed shape (it pins `-0.0`). `unconfirmed: true` appears only in `_placeholder/manifest.json`.

Go types live in `internal/harness/manifest.go` with JSON struct tags, so the manifest schema is code (plan: "JSON output defined as Go structs").

### 4. Redaction (`internal/harness/redact.go`)

`Redact(raw []byte, home string) (out []byte, n int)`; operates on the raw bytes with regexp replacement — never by decoding and re-encoding JSON — so byte fidelity of everything else is preserved.

- Pattern: any absolute path rooted in a home directory — the capturing user's `home` (from `os.UserHomeDir()`), or the generic forms `/home/<user>` and `/Users/<user>` — together with the rest of that path up to the closing JSON quote or whitespace. Each distinct matched path becomes `~/redacted-<n>` with `n` assigned in first-seen order within one capture (stable across the file; the project name never survives, which is what "redact project paths" means).
- Returns the replacement count, recorded as `redactions` in the manifest and printed per cell.
- Expected on this machine: `redactions=0` for all six cells (verified in the probe). The unit test exercises the mechanism with a synthetic payload containing `--instances`-style keys (`"projectPath": "/home/sahil/code/sahil87/tu"`, `"/Users/x/work/secret"`, and Claude's encoded form `-home-sahil-code-…` inside a path) and asserts the encoded-form case is **not** touched (it is not an absolute path; documenting the boundary is the point of the test).
- Hostnames, usernames outside a path, model names, and numbers are not redacted.

### 5. `tudiff placeholder` and the placeholder corpus

```
tudiff placeholder --source opencode --source copilot [--out harness/fixtures/_placeholder]
```

Generates deterministic, schema-derived fixtures for sources with no real capture. The schema is the observed v20 per-agent **claude-style** daily shape, chosen because the real empty opencode/copilot outputs use `totalCost` in `totals` (the codex `costUSD`/`models` variant is the one outlier, and codex is captured for real):

```json
{
  "daily": [
    {
      "cacheCreationTokens": 1000,
      "cacheReadTokens": 20000,
      "date": "2026-01-05",
      "inputTokens": 3000,
      "modelBreakdowns": [
        {
          "cacheCreationTokens": 1000,
          "cacheReadTokens": 20000,
          "cost": 0.5,
          "inputTokens": 3000,
          "modelName": "placeholder-opencode-model",
          "outputTokens": 400
        }
      ],
      "modelsUsed": ["placeholder-opencode-model"],
      "outputTokens": 400,
      "totalCost": 0.5,
      "totalTokens": 24400
    }
  ],
  "totals": { "cacheCreationTokens": …, "cacheReadTokens": …, "inputTokens": …, "outputTokens": …, "totalCost": …, "totalTokens": … }
}
```

- Three consecutive days (2026-01-05..07) with fixed values; `totals` are the arithmetic sums; `totalTokens` equals the sum of the four counters — internally consistent so tu's aggregation can be checked against it.
- Emitted from Go structs mirroring the observed schema, with keys sorted alphabetically and 2-space indentation to match ccusage's serializer layout. Apply verifies the layout choice by decoding the real `claude/daily.json` into the same structs, re-encoding, and diffing: the structural layout (key order, indentation) must match; number formatting differences (e.g. float precision) are acceptable and noted, since placeholders are shape fixtures, not byte oracles.
- `_placeholder/manifest.json` has `machine: "_placeholder"`, `ccusage_version: "20.0.19"` (the version the shape was derived from), `ccusage_path: ""`, an extra top-level `"derived_from": "dev-ws-sahil02/claude/daily.json"`, and every fixture `unconfirmed: true`. P3b flips a fixture to confirmed by re-running `tudiff capture` on a machine with real data and deleting the placeholder entry — never by hand-editing the placeholder file.
- The five-source ask in the row collapses to two (opencode, copilot) because codex, gemini, and kimi are real on this machine. Only sources with no real non-empty capture anywhere in `harness/fixtures/` get a placeholder; the generator refuses (`exit 1`) to write a placeholder for a source that already has a confirmed non-empty fixture, so the corpus cannot carry both.

### 6. Fake `ccusage` (`cmd/fakeccusage` → `bin/harness/ccusage`)

A single static executable configured only by environment variables, because its argv belongs to tu.

- `TUDIFF_FIXTURES` (required): an OS-path-list (`:`-separated) of fixture *machine* directories, searched in order, first hit wins — e.g. `harness/fixtures/dev-ws-sahil02:harness/fixtures/_placeholder`. Missing → stderr `fakeccusage: TUDIFF_FIXTURES not set`, exit 2.
- Request matching: argv is parsed as `<source> <period> [flags...]`; the key is `(source, period, sorted flags)` and must equal a manifest entry's `(source, period, args)` exactly. On a hit it writes the fixture's stdout bytes verbatim, the recorded stderr (if any) to stderr, and exits with the recorded `exit_code`. No fixture → stderr `fakeccusage: no fixture for argv [...]`, exit 2 — deliberately loud, because a Go port that sends ccusage an argv the TS binary never sent is itself a divergence the harness must surface.
- `--version` / `-v` as the sole argument prints `ccusage <ccusage_version>` from the first manifest found, exit 0.
- Every invocation is appended as one JSON line to `$TUDIFF_CALL_LOG` when set (`{"tool":"ccusage","argv":[…],"cwd":"…","matched":"dev-ws-sahil02/claude/daily.json"}`), so P4 can byte-compare the *sequence* of ccusage calls between the two binaries, not just their output.
- P4 handoff, recorded here because it constrains this design: the TS fetcher does **not** look up `ccusage` on `PATH` — it execs `dist/vendor/ccusage/bin/ccusage` when `dist/vendor/` exists, else `node_modules/.bin/ccusage`. P4 must therefore stage a copy of `bin/harness/ccusage` at `<staged-dist>/vendor/ccusage/bin/ccusage` for the TS side, while the Go side (V1) resolves vendor-first relative to `os.Executable()` then `PATH`. This is why the fake is a self-contained binary configured by env, not a shell shim.

### 7. Fake `git` (`cmd/fakegit` → `bin/harness/git`)

tu reaches `git` through `PATH` on both sides (`execFile("git", ["-C", dir, …])` in `sync.ts`; `execSync("git clone …")`/`execFileSync("git", ["clone", …])` and `git -C <dir> rev-parse --git-dir` in `cli.ts`), so a `PATH`-first fake intercepts it. The commands tu issues, which the fake must accept: `rebase --abort`, `add <user>/`, `status --porcelain <user>/`, `commit -m <msg>`, `pull --rebase origin main`, `push`, `rev-parse --git-dir`, `clone <url> <dir>`.

- Logs every invocation as a JSON line to `$TUDIFF_CALL_LOG` (`{"tool":"git","argv":[…],"cwd":"…"}`), the same file and shape as the fake ccusage.
- Responses come from `$TUDIFF_GIT_SCRIPT` (optional; JSON array of rules, first match wins): `[{"match": ["status","--porcelain"], "stdout": " M sahil/2026/x.jsonl\n", "stderr": "", "exit": 0}]`. `match` is a prefix match on argv after stripping a leading `-C <dir>`. No rule matched, or no script → stdout empty, stderr empty, exit 0.
- It performs no filesystem operations: the D11/B6 live-sync parity check uses real `git` against a temp bare repo; the fake exists for the deterministic, network-free matrix and for comparing the *sequence* of git calls between implementations.

### 8. Build recipes, corpus test, plan bookkeeping

`justfile` gains two recipes under the existing Go section; `go-build`, `go-test`, `go-lint` are unchanged and already cover the new packages via `./...`:

```just
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
```

- `src/go/internal/harness/corpus_test.go` walks `../../../harness/fixtures/*/manifest.json` (Go tests run with cwd = package dir) and asserts: every referenced file exists; `sha256` matches; stdout parses as JSON with a `daily` array whose length equals `days`; `first_date`/`last_date`/`empty` agree with the content; no string value contains `/home/`, `/Users/`, or `~/redacted-` followed by nothing; `unconfirmed: true` occurs only under `_placeholder/`; no source has both a confirmed non-empty fixture and a placeholder. This runs in the existing `go-build-and-test` CI lane through `just go-test` — **no CI workflow edit**.
- Apply runs `just harness-capture` on this machine and commits `harness/fixtures/dev-ws-sahil02/` (six fixtures + manifest) and `tudiff placeholder --source opencode --source copilot` output under `harness/fixtures/_placeholder/`. Expected totals: 8 fixture files, 2 manifests, roughly 55 KB.
- `fab/plans/sahil/26-09-15-go-port.md`: fill row P3a's **PR** column with `260915-r7dh-harness-fixture-capture` and a short landed status; correct D7's rationale to the observed state ("dev-ws-sahil02 has real claude, codex, gemini, kimi data; opencode and copilot are empty there — 2026-09-16"); narrow P3b's scope to "one machine with opencode data and one with copilot data". No other plan edits.
- `.gitignore` needs no change (`bin/` already covers `bin/harness/`). `fab/project/config.yaml` needs no change.

### Not in scope (explicitly)

- `tudiff run`, the argument matrix file, temp `$HOME`/conf variants, env matrix, staging `dist/` with the fake — all P4. `tudiff run` exists only as a stub printing `tudiff: run is not implemented (plan row P4)` to stderr, exit 1.
- Any `src/go/internal/{fact,source,…}` package (V1), any change under `src/node/`, `dist/`, `scripts/build.sh`, `release.yml`, `ci.yml`, the formula, README, `docs/site/`.
- Capturing `weekly`/`monthly`/`--since`/`--instances` cells: tu never sends them. The layout and manifest are general (`period`, `args`) so they can be added later without a schema change.
- Confirming opencode/copilot shapes — that is P3b, by a human on another machine.

## Affected Memory

- `harness/differential-harness`: (new) the `harness/fixtures/<machine>/` layout, manifest schema v1 and field semantics (`unconfirmed`, `empty`, `redactions`, `sha256`), the redaction rule, `tudiff capture`/`placeholder` contracts and ccusage-binary resolution order, the fake `ccusage` (`TUDIFF_FIXTURES` search list, strict no-fixture exit 2, `--version`) and fake `git` (`TUDIFF_GIT_SCRIPT` rules, default exit 0) contracts, the shared `TUDIFF_CALL_LOG` line shape, the P4 handoff about TS's fixed vendor path; Design Decisions: manifest is JSON not YAML (zero deps), fixtures are verbatim bytes (no re-serialization), placeholders use the claude-style shape, real empty captures are confirmed not unconfirmed, fakes are env-configured binaries not shell shims.
- `build/toolchain`: (modify) add `just harness-build` (→ `bin/harness/{tudiff,ccusage,git}`) and `just harness-capture`; note the first `src/go/internal/` package and that `go-test`/`go-lint` cover it with no CI change; fixture corpus validation runs inside `go-build-and-test`.
- `cli/data-pipeline`: (modify) record the live-verified ccusage v20 per-agent daily JSON shapes: claude/gemini/kimi/opencode/copilot use `totalCost` + `modelBreakdowns[]` + `modelsUsed[]`; codex uses `costUSD` + `models{}` + `reasoningOutputTokens` (which is why `toUsageTotals` reads `totalCost ?? costUSD`); an agent with no transcripts returns exit 0, `daily: []`, and `totals.totalCost: -0.0`; `date` is the label key for all six (confirming the existing bullet with a real capture instead of source reading).

## Impact

- **New files**: `src/go/cmd/tudiff/main.go`, `src/go/cmd/fakeccusage/main.go`, `src/go/cmd/fakegit/main.go`, `src/go/internal/harness/{manifest,redact,capture,placeholder,replay,calllog}.go` and their `_test.go` siblings plus `corpus_test.go`; `harness/fixtures/dev-ws-sahil02/{manifest.json,claude,codex,opencode,gemini,copilot,kimi}/daily.json`; `harness/fixtures/_placeholder/{manifest.json,opencode,copilot}/daily.json`.
- **Modified files**: `justfile` (two recipes), `fab/plans/sahil/26-09-15-go-port.md` (row P3a, D7, P3b), memory files per Affected Memory (hydrate).
- **Runtime/user impact**: none. Nothing ships; `tu` as installed is unchanged; no frozen surface is touched.
- **Data committed to a public repo**: `sahil87/tu` is public. The real fixtures carry this machine's actual daily spend and token counts for Claude Code, Codex, Gemini, and Kimi (Aug–Sep 2026) and the model names used (e.g. `claude-fable-5-1`, `gpt-5.6-sol`, `kimi-k3-fast`). This is what D7 decided ("collect fixtures … commit"), and no project paths or usernames are present; the change stays as specified. If that exposure is unwanted, the decision point is Assumption 3 — resolve it via `/fab-clarify` before apply (options: capture only claude as the plan originally assumed, or `--since` to narrow the window; both reduce corpus value).
- **CI**: `go-build-and-test` gains the harness packages' unit tests and the corpus test (sub-second). No new jobs, no new dependencies, no `go.sum`.
- **Dependencies**: none added. `npm ci` (dev `ccusage`) or a prior `npm run build` (vendored `dist/vendor/…`) is needed only on the machine running `tudiff capture`; CI never runs a capture.
- **Unblocks**: P4 (`harness-differential`), V1 (`fact-source-ccusage` unit tests against real fixtures).
- **Local verification at apply**: `just go-lint && just go-test && just harness-build`; `TUDIFF_FIXTURES=harness/fixtures/dev-ws-sahil02 bin/harness/ccusage claude daily --json | cmp - harness/fixtures/dev-ws-sahil02/claude/daily.json` (byte-identical); `bin/harness/ccusage kimi weekly --json` exits 2; `TUDIFF_CALL_LOG=/tmp/x bin/harness/git -C /tmp status --porcelain u/` exits 0 with an empty stdout and one log line; re-running `just harness-capture` produces a diff confined to `captured_at` (and to data if new transcripts landed).

## Open Questions

- None blocking. P4 handoff notes (not this change's work): (a) fixtures run through 2026-09-16, so P4's snapshot ("today") cases need a pinned clock both binaries honor — fixtures alone cannot fix "now"; (b) the TS side must be staged with the fake at `dist/vendor/ccusage/bin/ccusage` (see § 6).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `src/go/cmd/tudiff` with a `capture` subcommand; fixtures under `harness/fixtures/<machine-alias>/`; redaction of project paths; fake ccusage replayer and fake git; capture run here and committed; placeholders marked `unconfirmed: true` in a manifest | Stated verbatim in the row and in plan D6/D7 | S:95 R:70 A:95 D:95 |
| 2 | Confident | Capture whatever sources are real on this machine (claude, codex, gemini, kimi confirmed; opencode, copilot real-empty) and generate placeholders only for opencode and copilot, instead of the row's "five other sources" | Live probe on 2026-09-16 contradicts the plan's "Claude data only"; the user's wording "whichever agents data this machine has" is the operative instruction; real data beats placeholders and is exactly what D7 wants | S:80 R:85 A:90 D:85 |
| 3 | Confident | Commit real spend/token/model-name fixtures for four sources to the public repo, unmodified | D7 explicitly decides "collect fixtures … commit those fixtures"; no paths or usernames present; but pushed history is permanent and the plan expected Claude-only data, so this is the one row worth a second look before apply | S:85 R:30 A:80 D:85 |
| 4 | Confident | Capture matrix is the six ccusage subcommands × `daily` × `[--json]` only; layout keeps `period`/`args` general | tu only ever sends `<agent> daily --json` (spec § 215; every `cli.ts` call site passes `extraArgs=[]`); recording cells tu never sends adds no harness value, and the replayer's strict miss makes an unexpected Go-side argv visible | S:70 R:90 A:90 D:75 |
| 5 | Confident | Manifest is JSON (Go structs with tags), one per machine directory, regenerated wholesale per capture; `source` uses ccusage subcommand names | Zero-dependency posture (P2) rules out YAML; per-machine files keep P3b captures from other machines conflict-free; the replayer matches ccusage argv | S:55 R:85 A:85 D:75 |
| 6 | Confident | Fixture bytes are stored verbatim after regexp redaction on raw bytes; never decoded and re-encoded | Byte fidelity is the harness's purpose; `-0.0` and key order are real serializer traits a re-encode would erase | S:65 R:80 A:90 D:90 |
| 7 | Confident | Redaction target: absolute paths rooted in `$HOME`, `/home/<u>`, `/Users/<u>` → `~/redacted-<n>`; hostnames/model names untouched; expected 0 hits on this machine | Row says "redacts project paths"; default `daily --json` carries none (probed); the mechanism must still exist and be tested for `--instances`-style payloads | S:60 R:90 A:80 D:70 |
| 8 | Confident | Real empty opencode/copilot captures are `unconfirmed: false` with `empty: true`; placeholders live under `_placeholder/` and are the only `unconfirmed: true` entries; the generator refuses to shadow a confirmed non-empty fixture | An empty result is a genuine observed shape; separating "we saw nothing" from "we guessed the shape" is what lets the harness report placeholders separately (row text) | S:65 R:90 A:85 D:75 |
| 9 | Confident | Placeholders use the claude-style shape (`totalCost`, `modelBreakdowns[]`, `modelsUsed[]`), three fixed days, sums consistent, generated deterministically by `tudiff placeholder` | The real empty opencode/copilot outputs use `totalCost` in `totals`; codex is the sole `costUSD`/`models{}` outlier and is captured for real | S:60 R:90 A:75 D:70 |
| 10 | Confident | Fakes are separate stdlib-only Go mains (`cmd/fakeccusage`, `cmd/fakegit`) built to `bin/harness/{ccusage,git}`, configured via `TUDIFF_FIXTURES` (ordered dir list), `TUDIFF_CALL_LOG`, `TUDIFF_GIT_SCRIPT`; unknown ccusage argv exits 2 | Their argv belongs to tu, so env is the only channel; separate mains beat an argv[0]-dispatch trick (readability over cleverness); a loud miss is a harness signal | S:55 R:85 A:80 D:65 |
| 11 | Confident | Fake git logs and scripts responses only; it performs no filesystem or network work | D11/B6 parity uses real git against a temp bare repo; the fake's job is determinism plus a comparable call sequence | S:55 R:90 A:80 D:75 |
| 12 | Confident | Two new `just` recipes (`harness-build`, `harness-capture`); no CI workflow edit; corpus validation is an ordinary `go test` under the existing lane | `go test ./...`/`go vet ./...` already cover new packages; one definition of "test" (code-quality: minimum pathways) | S:60 R:95 A:90 D:85 |
| 13 | Confident | Shared package is `src/go/internal/harness` (first `internal/` package), `_test.go` siblings, ccusage resolved vendor → `node_modules/@ccusage/…` → `PATH` | D5 layout; constitution § Go Transition; `scripts/build.sh` shows where the native binary lives in each mode | S:60 R:85 A:90 D:80 |
| 14 | Confident | Update plan row P3a, D7's rationale, and P3b's scope in this change | P2 set the precedent for plan bookkeeping; D7's premise is demonstrably stale and P3b's ask shrinks to two sources | S:65 R:100 A:90 D:85 |
| 15 | Certain | No change to `src/node/`, `dist/`, build/release scripts, CI workflows, formula, README, `docs/site/`, or any surface in the plan's Goal | Row instruction + constitution § Go Transition + plan Not-a-goal | S:90 R:95 A:95 D:95 |

15 assumptions (2 certain, 13 confident, 0 tentative, 0 unresolved).
