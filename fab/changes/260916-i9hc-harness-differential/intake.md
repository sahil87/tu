# Intake: Harness Differential (`tudiff run`)

**Change**: 260916-i9hc-harness-differential
**Created**: 2026-09-16

## Origin

Plan row **P4** of `fab/plans/sahil/26-09-15-go-port.md` (the tu Go port), handed to `/fab-new` one-shot with the plan's standard context prefix. Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row P4. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Scope: tudiff run in src/go/cmd/tudiff — an argument matrix file; a temp $HOME with conf variants (single, multi, org.conf, legacy); an env matrix (NO_COLOR, TU_METRICS_REPO, TTY vs pipe via script); a fixed TZ plus one alternate TZ; byte-diff of stdout, stderr and exit code between node dist/tu.mjs and the Go binary, using the fake ccusage and fake git from P3a on PATH. The harness reads real captures from the gitignored harness/fixtures/<machine>/ when present and falls back to harness/fixtures/_placeholder/ otherwise (D7). Report the first divergence per case plus a summary count; every case is red at this point because the Go binary prints "unimplemented", and that count is the burndown. Wire just go-diff and a CI job that runs the placeholder matrix and uploads the report as an artifact without failing the build yet.

Plan context read for this intake: **Decisions** D2 (both trees in `main`, Go unshipped), D4 (TS feature freeze — this change touches no `src/node/` file), **D6** (the harness is the release gate: byte-diff of stdout/stderr/exit between `node dist/tu.mjs` and the Go binary over a fake `ccusage`, fake `git`, temp `$HOME`, argument matrix), **D7** (real captures local-only, `_placeholder/` committed, harness runs against local captures where they exist), D11 (metrics-repo format untouched); **Target architecture** (P4 is tooling in `src/go/cmd/tudiff` + `src/go/internal/harness`, not one of the pipeline packages — it exercises `cmd/tu` from the outside and must not depend on any of them). Also read: the P3a memory `docs/memory/harness/differential-harness.md` (fixture corpus, fake ccusage/git contracts, `TUDIFF_*` env vars, the "TS-side staging" handoff), `docs/memory/build/toolchain.md` (recipes, CI lanes), `docs/memory/configuration/config-system.md` (the conf cascade the variants must exercise), `docs/specs/usage.md` § CLI Grammar / Global Flags / Setup Commands / Exit Codes / Terminal width and color / Multi-Machine Mode.

**External surface**: none of the surfaces listed under the plan's Goal changes. `dist/tu.mjs`, `src/node/`, the formula, `tu.conf`/`org.conf` semantics and the metrics-repo layout are read, staged into temp dirs, or byte-compared — never edited. The only user-visible additions are the `tudiff run` subcommand (already stubbed by P3a), one `just` recipe, one CI job, and committed fixture data under `harness/`.

## Why

**The problem.** The port's bar is byte-identical output (D6, Output Stability clause). Phase 1 (V1/V2) and every Phase 2 row are gated on "harness cases green", and the plan's G1/G2/G3 gates read the harness report as their objective signal. Today there is no `tudiff run`: P3a built the corpus (`harness/fixtures/`), the replayers (`bin/harness/ccusage`, `bin/harness/git`) and the call log, but the driver that stages both binaries, crosses the argument matrix with conf/env/TTY/TZ axes, and byte-diffs the result exists only as a stub that prints `tudiff: run is not implemented (plan row P4)`. Without it V2's "every single-mode snapshot case green" has no definition, and the risk register's largest item ("output parity on the formatter, 1842 lines of byte-exact tables") has no detector.

**If we don't build it now.** Phase 1 would start writing Go against the specs with the TS binary as an informal oracle, and every parity check would be an ad-hoc `diff <(node dist/tu.mjs …) <(bin/tu …)` run by hand — unrepeatable, un-reviewable at G0, and blind to the axes (`NO_COLOR`, TTY width budget, TZ bucketing, the org/legacy conf cascade) where the TS behavior is subtle. The plan's whole shape — serial rows judged by a falling red count — needs the counter to exist before the first row that could move it.

**Why this approach.** The plan already decided the shape (D6): an external black-box differential over fakes, not unit tests of Go internals, because it replaces most of the 11.5k lines of TS string-assertion tests *for verifying the port* and lets the Go internals be redesigned freely (Target architecture) rather than transliterated. P3a deliberately left three seams for P4 to consume: `TUDIFF_FIXTURES` (ordered alias list — D7's local-first/placeholder-fallback is a one-line PATH-list), the fixed `dist/vendor/ccusage/bin/ccusage` slot the TS fetcher execs (so the fake is *staged* beside a copy of the bundle rather than looked up on PATH), and `TUDIFF_CALL_LOG` (so call sequences can be compared, not only output). This change is the consumer of those seams. Everything starts red by construction — `cmd/tu` prints `tu: not implemented (Go port in progress)` on stderr, exit 1, for everything but `--version` — so the first report is the burndown baseline, and the `--version` cases are the proof that a case *can* go green.

## What Changes

### 1. `tudiff run` — the driver (`src/go/cmd/tudiff/run.go`, new file)

`run` is dispatched from the existing `run(args, stdout, stderr) int` seam in `main.go` (replacing the stub branch). Stdlib only (`flag`, `os/exec`, `encoding/json`, `path/filepath`, `bytes`, `sync`), matching the harness posture (no `go.sum` yet). Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `--matrix <file>` | `harness/matrix.json` | The argument matrix (§2) |
| `--node <path>` | `dist/tu.mjs` | The TS oracle bundle; run as `node <staged copy>` with `node` from `PATH` |
| `--go <path>` | `bin/tu` | The Go binary under test |
| `--harness-bin <dir>` | `bin/harness` | Directory holding the fakes `ccusage` and `git` (built by `just harness-build`) |
| `--fixtures <alias>[,<alias>…]` | *(D7 resolution, §5)* | Explicit ordered alias list under `harness/fixtures/`; overrides the automatic choice |
| `--placeholder` | off | Force `_placeholder` only (what CI passes); mutually exclusive with `--fixtures` |
| `--report <dir>` | `bin/harness/report` | Report directory (§7); wiped at start |
| `--filter <substring>` | *(none)* | Run only cases whose expanded ID contains the substring |
| `--list` | off | Print the expanded case IDs (one per line) and exit 0 without running anything |
| `--jobs <n>` | `4` | Cases run concurrently; each case is fully isolated (own temp dir, own `HOME`s, own call logs), so parallelism is safe |
| `--timeout <dur>` | `60s` | Per-side wall-clock bound; a timeout is a red case with channel `timeout` |

Repo root is found via the existing `harness.FindRepoRoot(cwd)` (walk up to `package.json`) — every relative default above resolves against it, so `tudiff run` works from any subdirectory like `capture` does.

**Preflight** (all exit 2 with a one-line `tudiff: …` reason on stderr): matrix unreadable or invalid (§2 schema), `--node` file missing (`run npm ci && npm run build` hint), `--go` missing or not executable (`just go-build` hint), `--harness-bin` lacking `ccusage` or `git` (`just harness-build` hint), `node` not on `PATH`, `script` not on `PATH` when the expanded matrix contains any `io: tty` case, `harness/fixtures/_placeholder/manifest.json` absent, `--fixtures` naming an alias with no manifest.

**Exit code of `tudiff run`**: `0` when every executed case is green, `1` when any case is red (including `timeout`), `2` for usage/preflight errors. This makes the same command usable as the R3 gate later without change; P4's CI job neutralises the `1` with `continue-on-error` (§8).

### 2. The argument matrix — `harness/matrix.json` (new, committed)

JSON (stdlib decode; same choice as `manifest.json` and `TUDIFF_GIT_SCRIPT`). Each entry is a **case group**: one tu argv plus the axes it should be crossed with. Omitted axes take the single base value, so the file controls the case count explicitly instead of exploding into a full cross-product.

```json
{
  "schema": 1,
  "cases": [
    { "id": "version",        "args": ["--version"] },
    { "id": "help",           "args": ["--help"],  "io": ["pipe", "tty"] },
    { "id": "snapshot-all",   "args": [],
      "conf": ["single", "multi", "org", "legacy"],
      "env":  ["default", "nocolor", "envrepo"],
      "io":   ["pipe", "tty"],
      "tz":   ["fixed", "alt"] },
    { "id": "history-cc-window", "args": ["cc", "h", "--since", "2026-01-01", "--until", "2026-01-31"],
      "conf": ["single", "multi"], "tz": ["fixed", "alt"] },
    { "id": "usage-unknown-arg", "args": ["bogus"] }
  ]
}
```

| Axis | Values | Base (when omitted) | What it drives |
|------|--------|---------------------|----------------|
| `conf` | `single`, `multi`, `org`, `legacy` | `single` | Which `$HOME` skeleton is staged (§3) |
| `env` | `default`, `nocolor`, `envrepo` | `default` | `default`: neither var set; `nocolor`: `NO_COLOR=1`; `envrepo`: `TU_METRICS_REPO=git@example.invalid:harness/tu-metrics.git` (flips even `single` into multi mode through the env layer of the cascade) |
| `io` | `pipe`, `tty` | `pipe` | `pipe`: stdout and stderr captured separately through pipes; `tty`: run under a pseudo-terminal via `script` (§4) |
| `tz` | `fixed`, `alt` | `fixed` | `fixed`: `TZ=UTC`; `alt`: `TZ=Asia/Kolkata` (non-integer offset, the zone the local captures were bucketed in; a second zone is the plan's stated guard for the 31 `new Date(...)` sites) |

Validation at load: `schema` must be `1`; every `id` non-empty, unique, and a single path component (reuse `pathComponent` — the id becomes a directory name under the report); `args` present (may be empty); every axis value from the enumerations above; unknown keys rejected. Expanded **case ID** = `<id>/<conf>/<env>/<io>/<tz>`, e.g. `snapshot-all/multi/nocolor/tty/alt` — every axis always appears so IDs are stable when a group later gains an axis.

**Initial matrix contents** (the G0 checklist item "confirm the harness matrix covers the cases you care about" reviews this file; it is data, edited without code changes). Groups, by area:

- **Toolkit / HOME-independent**: `--version`, `-v`, `--help`, `help`, `help-dump`, `skill`, `shell-init bash`, `shell-init zsh`, `shell-init fish`, `shell-init` (missing shell → exit 2), `shell-init tcsh` (unknown → exit 2). Base axes only, plus `io: tty` for `--help`.
- **Snapshots**: `[]`, `cc`, `codex`, `co`, `oc`, `gemini`, `gem`, `copilot`, `cop`, `kimi`, `ki`, `w`, `m`, `cc m`, each with `conf` all four and `io` both; `[]` and `cc` additionally with `env` all three and `tz` both. `--json`, `-j`, `--csv`, `--md`, `--by-machine`, `-t`, `--metric tokens`, `--metric cost`, `--fresh` variants of `[]` and `cc` with `conf: [single, multi]`.
- **History**: `h`, `dh`, `wh`, `mh`, `cc h`, `cc mh`, `history`, each as (a) default window and (b) `--since 2026-01-01 --until 2026-01-31` (the placeholder dates; the default 3-month window renders empty against placeholders and populated against a local capture — both are legitimate cases) and (c) `--full`; `conf: [single, multi]`, `tz` both on the windowed variants; `--json`/`--csv`/`--md` on `h` and `cc mh`; `--by-machine` on `cc h` (supported) and `h` (warn-and-ignore path).
- **Leaderboard**: `lb`, `m lb`, `cc m lb`, `lbh`, `w lbh`, `--top 1` and `--top 0` (exit 2) variants, `-u other-user`, `-u all`, `-u harness-user`, `--json`/`--md` on `lb` and `lbh`; `conf: [single, multi, org, legacy]` (single is the `Error: lb requires multi mode` exit-1 path).
- **Multi-mode flags**: `-u other-user`, `-u all`, `-u` (missing value → exit 2), `--by-machine -u all`, `--sync` (fake git answers silently → `syncing metrics...` then table), `cc --sync`.
- **Setup commands**: `status`, `init-conf`, `init-conf` twice in one HOME is *not* a case (each case gets a fresh HOME); `init-metrics` (no URL; exit 1 in `single`), `init-metrics git@example.invalid:harness/tu-metrics.git`, `init-metrics a b` (exit 2), `sync` (exit 1 in `single`), `sync --dry-run`; all four `conf` values.
- **Usage errors**: `bogus`, `cc codex`, `cc --help`, `--json --csv`, `--watch --json`, `--dry-run`, `cc --dry-run`, `--top x`, `--metric`, `--metric foo`, `-t --metric cost`, `--since 2026-13-01`, `--since 2026-02-01 --until 2026-01-01`, `--interval 3`, `--interval abc`, `--until` (missing value).
- **Excluded**: `--watch`/`-w` in any form (interactive loop — B7 defines its own frame-capture approach), `update` (Homebrew and network), anything needing a real git remote (B6's live-sync parity check is a separate, real-git procedure per D11).

Target size for the initial file: roughly 250–350 expanded cases, a few minutes sequential and well under a minute at `--jobs 4` once the Go side answers quickly (the TS side is ~0.3 s per cached call on the reference machine). The `--version` group is the one expected-green group at P4.

### 3. Temp `$HOME` skeletons — the four conf variants (`src/go/internal/harness/homes.go`, new)

For each expanded case the harness creates `<tmp>/<case>/node/home` and `<tmp>/<case>/go/home` — **two** fresh copies of the variant skeleton, one per side, so the TS run's cache (`~/.tu/cache/*.json`, 60 s TTL) and its multi-mode day-file writes (`a plain tu in multi mode is a write`) can never be read by the Go run and mask a divergence or change its call sequence. Skeletons are generated in code from a small committed seed, not committed as dot-directories:

| Variant | Files staged under `$HOME` | Cascade path exercised |
|---------|----------------------------|------------------------|
| `single` | *(nothing)* — no `.config/`, no `.tu/` | Defaults only → single mode; `status` prints `Mode:        single (no ~/.config/tu/tu.conf)` |
| `multi` | `.config/tu/tu.conf` (contents below) + seeded `.tu/metrics_repo/` | `tu.default.conf < tu.conf`; multi mode |
| `org` | `.config/tu/org.conf` (same contents) + seeded `.tu/metrics_repo/`; **no** `tu.conf` | `tu.default.conf < org.conf`; org-only layout (`status` prints `Org config:` and omits `Config:`) |
| `legacy` | `.tu.conf` (same contents) + seeded `.tu/metrics_repo/`; **no** `.config/tu/` | Legacy fallback → one stderr line `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` |

The conf contents, identical across `multi`/`org`/`legacy` so the only variable is *where* the file sits:

```
version = 2
metrics_repo = git@example.invalid:harness/tu-metrics.git
metrics_dir = ~/.tu/metrics_repo
machine = harness-machine
user = harness-user
auto_sync = true
```

`machine`/`user` are pinned so the `$HOSTNAME`/`$USER` sentinels do not make multi-mode output machine-specific. (`single` has no conf, so single-mode output that includes the host — `--by-machine`'s one-column legend, `status`'s `Machine:` line — still equals across the two sides because both run on the same host; the *report* is simply not portable across machines for those cases, which is acceptable and noted in the report header.)

**Seeded metrics repo** — a committed, deterministic mini-clone at `harness/metrics-repo/` copied into `$HOME/.tu/metrics_repo/` for the three multi variants (the metrics-dir guard is an `existsSync` on the directory; no `.git/` is needed and none is created — the fake `git` answers every call anyway). Layout follows the spec exactly (`{user}/{year}/{machine}/{tool}-{date}.jsonl`, one `UsageEntry` JSON line each) and uses the placeholder dates so it composes with `_placeholder/`:

```
harness/metrics-repo/
  harness-user/2026/harness-machine/cc-2026-01-05.jsonl   # own machine, lower cost than the fixture → live wins (max-merge)
  harness-user/2026/harness-machine/cc-2026-01-06.jsonl   # own machine, HIGHER cost than the fixture → stored wins
  harness-user/2026/other-box/cc-2026-01-06.jsonl         # own user, other machine
  harness-user/2026/other-box/codex-2026-01-07.jsonl
  other-user/2026/laptop/cc-2026-01-05.jsonl              # second profile → lb/lbh have two rows, -u other-user has data
  other-user/2026/laptop/gemini-2026-01-06.jsonl
  docs/README.md                                          # never read as a user (spec: docs/ excluded)
```

Example line (`UsageEntry` = `label` + the six `UsageTotals` fields): `{"label":"2026-01-06","totalCost":0.75,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"cacheReadTokens":20000,"totalTokens":24400}`. The two own-machine files straddle the fixture's `0.5` so the own-machine max-merge and the never-shrink write path both get one case each. B3 extends this seed for its own cases; P4 only lays it down.

### 4. Execution model — staging, environment, TTY (`src/go/internal/harness/diff.go`, new)

**Oracle staging (once per run).** `dist/` is the shipped artifact and is never mutated. The harness copies `dist/tu.mjs` to `<tmp>/oracle/dist/tu.mjs`, `tu.default.conf` to `<tmp>/oracle/dist/tu.default.conf` (the bundled layout — `findDefaultConf` checks beside the bundle first, exactly as in the brew bottle), and `bin/harness/ccusage` to `<tmp>/oracle/dist/vendor/ccusage/bin/ccusage` (mode `0755`) — the fixed path the TS fetcher execs when `dist/vendor/` exists, per the P3a "TS-side staging" handoff. The Go binary is used in place; it resolves `ccusage` vendor-first relative to `os.Executable()` (no `bin/vendor/` exists) then `PATH`, where the fake is first.

**Child environment (built from scratch, never inherited).** Exactly:

| Variable | Value |
|----------|-------|
| `PATH` | `<abs bin/harness>:<harness process's PATH>` — fakes first; `node` still reachable |
| `HOME` | The side's staged skeleton (§3) |
| `TZ` | `UTC` or `Asia/Kolkata` per axis |
| `LANG`, `LC_ALL` | `C.UTF-8` (the `📊` heading and `Δ`/`→` glyphs must encode identically) |
| `TERM` | `xterm-256color` — `tty` cases only (`script` and Node's tty layer read it) |
| `TUDIFF_FIXTURES` | The resolved alias list (§5) as an OS path list of absolute dirs |
| `TUDIFF_CALL_LOG` | `<report>/cases/<case>/<side>.calls.jsonl` |
| `NO_COLOR` | `1` — `nocolor` axis only |
| `TU_METRICS_REPO` | `git@example.invalid:harness/tu-metrics.git` — `envrepo` axis only |

Everything else from the developer's shell is dropped, so an exported `TU_METRICS_REPO` or `NO_COLOR` (both are known to leak into this repo's tests) cannot tilt a case. `TUDIFF_GIT_SCRIPT` is unset in P4 — the fake `git` is the silent exit-0 stub for every call; B6 scripts it.

**Working directory**: `<tmp>/<case>/<side>/` (not the repo root), so neither binary sees the checkout.

**Pipe cases**: `node <staged tu.mjs> <args>` and `<go binary> <args>` run with stdout and stderr captured into separate buffers; stdin is `/dev/null`. Captured: `stdout`, `stderr`, `exit`.

**TTY cases** (`io: tty`) run under a pseudo-terminal via `script` (util-linux on Linux CI; BSD on macOS). Because a pty merges stdout and stderr into one stream and `script`'s exit-code flag differs between the two implementations, the harness uses one portable wrapper for both:

```
script -q [-e -c "<wrapper>" /dev/null | /dev/null sh -c "<wrapper>"]
wrapper: stty cols 120 rows 40; <cmd> <args>; printf '\n__TUDIFF_EXIT=%s\n' "$?"
```

`stty cols 120 rows 40` pins the pty size (a pty spawned without a controlling terminal, as in CI, reports 0 columns, which the TS width budget would not fall back from) so `process.stdout.columns` is `120` on the TS side and the Go side sees the same `TIOCGWINSZ`. The transcript is captured as a single channel `tty`; the trailing `__TUDIFF_EXIT=<n>` sentinel is parsed off the end into `exit` and removed before comparison. util-linux vs BSD is detected once per run by `script --version` succeeding; the flavour is recorded in the report header. `\r\n` line endings from the pty are kept verbatim (identical on both sides). Captured: `tty`, `exit`.

**Call logs** are captured for both sides as an informational channel `calls` (see §6).

**Clock**: neither side takes a clock injection, and snapshot displays are "today". The two sides run back-to-back inside one case; the harness records the local date (in the case's `TZ`) before the first and after the second run and re-runs the case once if they differ, so a midnight rollover cannot produce a false red. With placeholder fixtures every snapshot is legitimately empty (January data, September today) — that is a real, deterministic case, not a defect; local captures populate it.

### 5. Fixture resolution (D7) — `src/go/internal/harness/fixtures.go` (new, small)

Default alias list: `harness/fixtures/<os.Hostname()>/` **if** it contains a `manifest.json`, followed by `harness/fixtures/_placeholder/`; otherwise `_placeholder` alone. `--fixtures a,b` replaces the list (each must have a manifest); `--placeholder` forces `_placeholder` alone (CI). The resolved list is printed in the report header (`fixtures: dev-ws-sahil02, _placeholder` / `fixtures: _placeholder`). Per case, after the run, the harness reads the two call logs' `matched` fields (`<alias>/<file>`) and looks each up in that alias's manifest: a case that replayed any `unconfirmed: true` fixture is flagged `unconfirmed` in the report (a separate column, never a colour) — this is the "harness reports them separately" clause of D7 and what R3 later turns into a gate failure.

### 6. Comparison — first divergence per case

Channels compared byte-for-byte, in this order, stopping at the first differing channel: `pipe` cases → `exit`, `stdout`, `stderr`; `tty` cases → `exit`, `tty`. A case is **green** when every compared channel is identical, **red** otherwise, **timeout** (counted as red) when either side exceeds `--timeout`. For the first differing byte-channel the report gives the byte offset, the 1-based line number, and a `%q`-quoted excerpt of up to 40 bytes on each side starting at the first differing byte, e.g.

```
RED   snapshot-all/multi/nocolor/pipe/fixed   exit: node=0 go=1
RED   help/single/default/pipe/fixed          stdout @0 (line 1): node="Usage: tu [source] [period] [display]\n\nSourc" go=""
GREEN version/single/default/pipe/fixed
```

`exit` divergence is reported as the two codes. The `calls` channel (both sides' `TUDIFF_CALL_LOG`, compared as the **sorted** set of `{tool, argv}` pairs — the TS fetcher issues its six `ccusage` calls through `Promise.all`, so raw order is nondeterministic) is **informational**: a difference is printed on the case's line as `calls: node=6 go=0 (differs)` but does not make a green case red. Making it a compared channel is a later decision (open question).

### 7. Report — `<report>/report.txt`, `<report>/report.json`, `<report>/cases/`

`report.txt` is what `tudiff run` also streams to its stdout: a header (timestamp UTC, node and go binary paths and versions — `node --version`, `<go> --version` — fixtures list, `script` flavour, matrix path and expanded count, `--filter`), one line per case in matrix order (`GREEN`/`RED`/`TIMEOUT` + case ID + first-divergence detail + `unconfirmed` marker where applicable), then the summary block:

```
tudiff: 312 cases — 2 green, 310 red, 0 timeout   (fixtures: _placeholder; 118 cases replayed unconfirmed fixtures)
  by conf:  single 3/140  multi 0/84  org 0/44  legacy 0/44
  by io:    pipe 2/236    tty 0/76
```

(`green/total` per axis value.) The last line is the burndown number the plan tracks. `report.json` carries the same data structured (`schema: 1`, header fields, `cases[]` with id, axes, status, channel, offset, line, excerpts, exit codes, durations, `unconfirmed`, `calls_differ`), for later charting. `cases/<id>/` (slashes in the case ID become directories) holds the raw per-side captures — `node.stdout`, `node.stderr`, `node.exit`, `node.tty`, `node.calls.jsonl`, and the `go.*` twins — so a red can be inspected with `diff` without re-running. `<report>` is wiped at the start of a run and lives under gitignored `bin/`.

### 8. Wiring — `justfile` and `.github/workflows/ci.yml`

`justfile` (after `harness-capture`):

```make
# Byte-diff node dist/tu.mjs against bin/tu over harness/matrix.json (plan row P4).
# Exit 1 while any case is red — the count is the port's burndown, not a gate yet.
go-diff *ARGS: build go-build harness-build
    bin/harness/tudiff run {{ARGS}}
```

`build` (esbuild + vendor) needs `npm ci` to have run; the recipe does not install dependencies.

`ci.yml`: a new job **`go-diff`** beside `go-build-and-test` — checkout, `setup-node` 20 (same pinned SHA), `setup-go` (`go-version-file: src/go/go.mod`, `cache: false`), `setup-just` (same pinned SHA), `npm ci`, then

```yaml
      - name: Differential harness (placeholder matrix)
        run: just go-diff --placeholder
        continue-on-error: true   # every case is red until Phase 1 lands; R3 removes this and adds the job to ci-gate

      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4
        if: always()
        with:
          name: tudiff-report
          path: bin/harness/report/
```

`ci-gate`'s `needs` list is **unchanged** (`[build-and-test, go-build-and-test]`), so the job can neither block a merge nor turn the gate red; the header comment names R3 as the change that flips both. The upload-artifact SHA is the same v4 pin the sibling toolkit repos use.

### 9. Tests (`_test.go` siblings)

- `matrix_test.go`: load/validate (bad schema, duplicate id, unknown axis value, unknown key → error naming the case), expansion order and ID format, base-axis defaults, `--filter` semantics; a test that loads the committed `harness/matrix.json` and asserts it validates and expands to at least one case per axis value.
- `homes_test.go`: each variant stages exactly the files in the §3 table (and nothing else), conf bytes are identical across `multi`/`org`/`legacy`, the seed copies completely, two stagings of one case share no path.
- `diff_test.go`: comparison over synthetic captures — identical → green; exit differs → `exit` reported first; stdout differs at byte N → offset/line/excerpts exactly; tty sentinel parsing (`__TUDIFF_EXIT=` present/missing); sorted-call-set comparison ignores order.
- `fixtures_test.go`: D7 resolution with and without a hostname alias present in a temp fixture root; `--placeholder` and `--fixtures` precedence; the `unconfirmed` flag derived from a call log against a manifest.
- `run_test.go` (cmd): preflight errors and their exit-2 messages; `--list` prints IDs without touching the report dir; an **end-to-end smoke** that builds nothing but uses two tiny stand-in "binaries" written by the test (shell scripts printing fixed output) as `--node`/`--go` to assert a green and a red case land in `report.json` with the expected first divergence. The stub test `TestRunStub` is replaced.
- The existing `TestCorpus` gains nothing; the seed under `harness/metrics-repo/` is validated by `homes_test.go` (each `.jsonl` line decodes into the seven `UsageEntry` fields).

No `src/node/` test is added or changed (D4).

## Affected Memory

- `harness/differential-harness`: (modify) replace the "tudiff run stub" requirement with the `run` contract — flags, preflight/exit codes, matrix schema and axes, the four `$HOME` variants and their conf bytes, the seeded metrics repo, oracle staging, the from-scratch child environment, the `script` wrapper and `__TUDIFF_EXIT` sentinel, the D7 alias resolution and `unconfirmed` flagging, the channel order and first-divergence format, the report layout, and the informational `calls` channel; add Design Decisions for "JSON case groups with axis defaults over a cross-product", "two HOMEs per case", "calls informational, not compared", "no clock injection — date-rollover re-run", and "`stty` inside the pty".
- `build/toolchain`: (modify) add the `go-diff` recipe (depends on `build go-build harness-build`) and the `go-diff` CI job — `continue-on-error`, `upload-artifact` (pinned v4) of `bin/harness/report/`, **not** in `ci-gate`'s needs, R3 named as the change that flips it; note `harness/matrix.json` and `harness/metrics-repo/` as committed harness data.
- `configuration/config-system`: (no change) — the variants exercise the documented cascade; nothing about it changes.

## Impact

**New files**: `src/go/cmd/tudiff/run.go` (+ `run_test.go`), `src/go/internal/harness/{matrix,homes,diff,fixtures,report}.go` (+ `_test.go` siblings), `harness/matrix.json`, `harness/metrics-repo/**` (six `.jsonl` day-files + `docs/README.md`).
**Modified**: `src/go/cmd/tudiff/main.go` (dispatch `run` → `runRun`, usage text line for `run` updated), `src/go/cmd/tudiff/main_test.go` (drop `TestRunStub`), `justfile` (`go-diff`), `.github/workflows/ci.yml` (`go-diff` job; `ci-gate` untouched), the two memory files above.
**Untouched by construction**: `src/node/**`, `dist/`, `package.json`, `scripts/`, `Formula`, `docs/specs/**`, `harness/fixtures/**` (read-only; `_placeholder` unchanged), `src/go/cmd/tu/main.go` (the red baseline is the point), `.gitignore` (report lives under already-ignored `bin/`; `harness/metrics-repo/` and `harness/matrix.json` are meant to be committed and match no ignore rule).
**Dependencies**: none added to `go.mod`. Runtime prerequisites for `just go-diff`: `node` ≥ 18 on `PATH`, `npm ci` done (for `build`), `script` (util-linux or BSD) for `tty` cases.
**Plan doc**: the P4 row's Status cell is updated in the post-merge plan commit, following the pattern of P0–P3a (their status edits were separate `plan:` commits after merge), not inside this change.
**Constitution**: Go Transition article applies (`_test.go` siblings, unshipped); Principle I — harness is verification tooling for the port, not a tu feature; Output Stability is what the harness *measures*, nothing here changes output.

## Open Questions

- Should the `calls` channel become a compared (red-making) channel once V1 lands, and if so as a sorted set or in issue order per tool? Left informational here so a TS/Go fetch-ordering difference cannot mask an otherwise-green case.
- G0 reviews `harness/matrix.json` for coverage; the initial file is the agent's best reading of the spec's surface, and rows may be added or trimmed there without code changes.
- BSD `script` on macOS is supported via the sentinel wrapper but only the util-linux path runs in CI; the first maintainer run on the Mac mini (R2) confirms the flavour detection.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Fixture alias resolution follows D7 literally: `harness/fixtures/<hostname>/` when its manifest exists, then `_placeholder`; `--fixtures` overrides, `--placeholder` forces placeholder-only; unconfirmed replays are flagged per case, not coloured | Plan D7 and the P3a memory spell out the local-first/placeholder-fallback and "report unconfirmed separately"; `TUDIFF_FIXTURES` already takes an ordered list | S:85 R:90 A:90 D:85 |
| 2 | Certain | The TS oracle is staged as a copy (`tu.mjs` + `tu.default.conf` beside it + the fake at `dist/vendor/ccusage/bin/ccusage`) in a temp dir; `dist/` is never mutated | P3a memory's "TS-side staging" handoff names the fixed vendor slot; `findDefaultConf` checks beside the bundle first; `dist/` is the shipped artifact | S:60 R:85 A:90 D:85 |
| 3 | Certain | Go binary `bin/tu`, oracle `dist/tu.mjs` via `node` on PATH, fakes from `bin/harness` prepended to PATH; all defaults resolve against the repo root found by walking to `package.json` | Existing recipes/paths (`go-build`, `harness-build`, `FindRepoRoot`) fix every location | S:70 R:90 A:90 D:85 |
| 4 | Certain | Child environment is built from scratch (PATH, HOME, TZ, LANG/LC_ALL=C.UTF-8, TERM for tty, TUDIFF_*, plus the axis vars); nothing inherited | Deterministic byte-diff requires it; `TU_METRICS_REPO`/`NO_COLOR` are known to leak from the developer shell into this repo's tests | S:60 R:90 A:85 D:80 |
| 5 | Certain | `just go-diff *ARGS` depends on `build go-build harness-build`; CI job `go-diff` runs `just go-diff --placeholder` with `continue-on-error: true`, uploads `bin/harness/report/` via `upload-artifact` pinned to the v4 SHA the sibling repos use, and is **not** added to `ci-gate` needs | The row says "without failing the build yet"; `ci-gate` is the only required check; the sibling pin exists | S:85 R:90 A:90 D:85 |
| 6 | Certain | Placeholder dates stay `2026-01-05..07`; snapshots run against placeholders are legitimately empty and windowed history cases (`--since 2026-01-01 --until 2026-01-31`, `--full`) exercise the data | Fixture bytes are sha-pinned and committed; shifting dates would break the corpus test and the "verbatim bytes" decision | S:70 R:90 A:90 D:90 |
| 7 | Certain | Plan doc P4 status is updated in the post-merge `plan:` commit, not inside this change | P0–P3a each got a separate status commit after merge (`8b04365`) | S:60 R:95 A:90 D:85 |
| 8 | Confident | Matrix file is `harness/matrix.json`: JSON, `schema: 1`, case groups with optional axis arrays and base defaults (`single/default/pipe/fixed`), expanded case ID `<id>/<conf>/<env>/<io>/<tz>` | Row says only "an argument matrix file"; JSON matches the harness's stdlib-only precedent (manifest, git script); explicit axes keep the count controllable and the file reviewable at G0 | S:55 R:85 A:80 D:65 |
| 9 | Confident | Every case stages two fresh `$HOME`s (one per side) from the variant skeleton | The TS side writes `~/.tu/cache` and multi-mode day-files; sharing a HOME would let the Go side read TS state and mask divergences | S:60 R:80 A:85 D:80 |
| 10 | Confident | Conf variants share one conf body (`metrics_repo = git@example.invalid:…`, `metrics_dir = ~/.tu/metrics_repo`, `machine = harness-machine`, `user = harness-user`, `version = 2`, `auto_sync = true`) placed at `tu.conf` / `org.conf` / `.tu.conf`; `single` has no files | Only the file's location should vary between variants; pinned labels defeat the `$HOSTNAME`/`$USER` sentinels; `.invalid` TLD can never resolve | S:70 R:80 A:80 D:70 |
| 11 | Confident | A minimal committed metrics-repo seed (`harness/metrics-repo/`, two users, two machines, own-machine files straddling the fixture cost) is copied into the multi variants; no `.git/` created | Multi-mode cases need repo data to be non-trivial (lb/lbh need ≥2 users; max-merge and never-shrink each need one case); the dir guard is `existsSync`; B3 extends it | S:45 R:80 A:70 D:55 |
| 12 | Confident | TTY cases run under `script` with a portable wrapper (`stty cols 120 rows 40; cmd; printf '\n__TUDIFF_EXIT=%s\n' "$?"`), merged transcript compared as one `tty` channel, exit parsed from the sentinel; util-linux vs BSD detected by `script --version` | Row names `script`; a pty merges stdout/stderr; the two `script`s disagree on the exit-code flag; an unsized pty reports 0 columns which the TS width fallback (`?? 80`) does not catch | S:70 R:80 A:70 D:70 |
| 13 | Confident | `TZ` fixed = `UTC`, alternate = `Asia/Kolkata` | Plan asks for "a fixed TZ plus one alternate" to shake the 31 `new Date` sites; Kolkata has a half-hour offset and is where the local captures were bucketed | S:60 R:90 A:60 D:50 |
| 14 | Confident | Compared channels: pipe → `exit`, `stdout`, `stderr`; tty → `exit`, `tty`; the call log is informational (sorted `{tool, argv}` set) and never reddens a case | Row lists stdout/stderr/exit; the TS fetcher's `Promise.all` makes raw call order nondeterministic, so counting it would flake | S:65 R:85 A:80 D:70 |
| 15 | Confident | First-divergence line = channel, byte offset, line number, 40-byte `%q` excerpts per side; `report.txt` streamed to stdout, `report.json` + `cases/<id>/` raw captures under `--report` (default `bin/harness/report`, wiped per run) | Row asks for first divergence per case + summary; raw captures make a red inspectable without re-running; `bin/` is already gitignored | S:65 R:90 A:80 D:70 |
| 16 | Confident | `tudiff run` exits 0 all-green, 1 any-red/timeout, 2 usage/preflight | Lets the identical command become the R3 gate; P4 neutralises the 1 in CI only | S:50 R:90 A:85 D:80 |
| 17 | Confident | Initial matrix (~250–350 cases): toolkit commands, all sources/periods snapshots, windowed/full history, leaderboards, multi flags, setup commands incl. `init-metrics`/`sync --dry-run`, usage errors; excludes `--watch` and `update` | Derived from spec § Grammar/Flags/Setup/Exit codes; watch is B7's, `update` needs brew; G0 reviews the file | S:70 R:95 A:75 D:70 |
| 18 | Confident | Per-side timeout 60 s → red with channel `timeout`; cases run with `--jobs 4` by default, each fully isolated; `--filter` and `--list` provided | Bounded runs for CI; isolation makes parallelism safe; filter/list are the developer loop | S:40 R:90 A:85 D:80 |
| 19 | Confident | No clock injection; the harness re-runs a case once if the local date changed between the two sides' runs | Node offers no fake clock without a dependency; back-to-back runs make rollover the only flake source | S:30 R:90 A:80 D:75 |
| 20 | Certain | Change type `feat` (P2/P3a precedent), all new Go code in `internal/harness` + `cmd/tudiff`; no new packages under the Target-architecture list | The harness is tooling around `cmd/tu`, not a pipeline stage | S:70 R:90 A:90 D:85 |

20 assumptions (8 certain, 12 confident, 0 tentative, 0 unresolved).
