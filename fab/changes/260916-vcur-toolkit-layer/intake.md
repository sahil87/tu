# Intake: Toolkit Layer — help, help-dump, update, shell-init, skill (Go port row B8)

**Change**: 260916-vcur-toolkit-layer
**Created**: 2026-09-17

## Origin

One-shot `/fab-new` invocation, handed over from the Go-port plan's queue (plan row B8, the first Phase 2 row after gate G1 returned `GO` in cycle 2 — commit `bcc2f88`):

> Context: fab/plans/sahil/26-09-15-go-port.md, row B8. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build help-dump (flat, commands: [], byte-identical --help), update (brew gate, HOMEBREW_NO_ASK, no timeout on upgrade, --skip-brew-update), shell-init, skill via go:embed plus a build drift guard, and completions for bash/zsh/fish. Audit each against shll standards <name> and record the shll version audited.

No prior discussion in this conversation. Sources read to ground every value below: the plan's Goal, Decisions (D1–D13), Target architecture table, § G1 review protocol, § Risks and § Out of scope; the Go-port memories `docs/memory/go-port/command-edge.md` (the placeholder row map assigning `help`/`-h`/`--help`, `help-dump`, `skill`, `shell-init`, `update` and `--skip-brew-update` to B8), `docs/memory/go-port/config-and-setup.md` (the embedded `tu.default.conf` + drift-guard precedent, the `CloneStep` edge-execution pattern, `internal/sync`'s `Exec` git driver) and the code they describe (`src/go/cmd/tu/main.go`, `src/go/internal/command/{parse,request,run}.go`, `src/go/internal/config/defaults{,_test}.go`); the TS memories `docs/memory/build/toolchain.md` (help-dump producer, `__SKILL_MD__` embed + post-build drift guard, the toolkit-standards posture and the audit ledger, the `HOMEBREW_NO_ASK` and no-timeout decisions) and `docs/memory/cli/data-pipeline.md` (non-data dispatch order, exit-code convention, the `shell-init`/`skill` requirements); the spec `docs/specs/usage.md` § Toolkit Contracts (audited against shll v0.1.32 on 2026-09-16 — the version this intake re-audits), § Exit Codes and the DC-04/DC-09 markers; the shipped TypeScript — `src/node/core/cli.ts` (`FULL_HELP`, `SHORT_USAGE`, `main()`'s help check and non-data dispatch block, `runHelpDump`, `runSkill`, `runShellInit`, `SHELL_INIT_USAGE`, `runUpdate`), `src/node/core/help-dump.ts` (`buildHelpDoc`, `extractUsage`), `src/node/core/skill.ts`, `src/node/core/completions.ts` (the three script constants), `scripts/build.sh` (the `--define` embeds and the `cmp` drift guard), `package.json` (`description`); the five standards `shll standards help-dump`, `update`, `shell-init`, `skill`, `version` read from the installed **shll v0.1.32** on 2026-09-17; the sibling Go implementations the plan says to copy (D5) — `idea/src/cmd/idea/{help_dump,skill,shell_init,update}.go`, `idea/src/internal/idea/update.go`, `idea/scripts/sync-skill.sh`, `idea/src/cmd/idea/skill_test.go`, `wt/src/internal/update/update.go` (the SIGTERM-graceful `newBoundedBrewCmd` helper); and a full `just go-diff --placeholder` run on this worktree (2026-09-17: **134 green of 368**, every B8 case red with the placeholder signature), whose node-side captures under `bin/harness/report/cases/` supply the byte references quoted in §9.

Plan context that shapes this change:

- **D2 / Goal** — Go lands dark; the formula is untouched. B8 makes the Go binary answer the toolkit contracts for real: `help`/`-h`/`--help`, `help-dump`, `skill`, `shell-init bash|zsh|fish` (plus its two usage errors), and `update` (with `--help`, the brew gate and `--skip-brew-update`). Every one of these is a frozen external surface — the CLI grammar, `--help` text, exit codes and the toolkit contracts are all named in the Goal — so B8 reproduces the TypeScript bytes; it does not "fix" anything it finds (see Open Questions for what it found).
- **Target architecture** — a new leaf package `internal/toolkit` owns `--version` (the line format; the `-ldflags` var stays in `main`), `help-dump`, `update`, `shell-init`, `skill` and the completions; `cmd/tu` stays the only writer. `toolkit` is the last package in the architecture table without code.
- **D5** — copy the sibling shape (`idea`/`hop`/`wt`): committed embedded skill copy + sync script + drift-guard test, `/Cellar/`-resolved brew gate, SIGTERM-graceful bounded brew metadata calls, `json.MarshalIndent`-style envelope. Copy, do not share (§ Out of scope).
- **D6** — the harness is the gate. §2 enumerates the 10 case IDs that flip green (134 → 144). `update` has no harness group by design (it spawns brew), so its parity is pinned by unit tests against a fake brew driver and by the deterministic non-Homebrew path.
- **Constitution § Toolkit Standards** — the row asks for an audit of each surface against `shll standards <name>` and the shll version recorded. §8 is that audit; the version is **shll v0.1.32**. The two conformance findings it turned up are frozen-surface matters and go to Open Questions, not into code.
- **G1 follow-through** — G1 (cycle 2, `GO`) reviewed V1+V2+B1+B2 on package boundaries, one group-by, no result globals, typed errors, table-driven/golden tests. B8 adds a package that execs (`brew`) below `cmd/tu`; §4 keeps that on the `internal/sync` `Exec` driver precedent (an injected driver interface, streams passed through, nothing printed by the package) so the next review sees the same shape.

## Why

The toolkit layer is what makes the Go binary a *toolkit member* rather than a data pipeline: `shll version`/`shll doctor` probe `--version` (done in P2), shll.ai's cron pulls `help-dump`, `shll update` delegates to `update` and discovers `--skip-brew-update` from `update --help`, `shll shell-init` composes `shell-init`, and `shll skill tu` streams `skill`. R1 (the release pipeline) uploads the Go binary as a dogfood asset and R2 installs it on maintainer machines; a Go binary without this layer would answer `shll`'s every probe with `tu: not implemented (Go port in progress)`, exit 1 — which the `shell-init` composer would (correctly) drop, the help-dump puller would treat as a failed capture, and `shll update` would report as a failed run. B8 is therefore the last dependency of R1 in the plan (`R1` depends on `B8`).

It is also the cheapest byte-exact win left in single mode: five surfaces whose TypeScript output is either a static string (help, the three completion scripts, the skill bundle) or a deterministic assembly of static strings (help-dump), plus one interactive command (`update`) whose wrapper messages are fixed and whose subprocess argv the standard freezes. The risk is not logic but transcription: the completion scripts carry `${...}`, backslash-continuations and backticks that the TS template literals escape (`\${`, `\\`, `\``) and that a Go source string would have to un-escape by hand, and `encoding/json` escapes `<`/`>`/`&` by default where `JSON.stringify` does not — the `--help` text contains `<date>`, `<m>`, `<n>`, `<s>`, `<user>`, `<sh>` and `[url]`. Both traps are closed by construction below (files embedded verbatim; `SetEscapeHTML(false)`).

Why now: the queue order (§ Queue handoff) puts B8 right after G1 and before R1, and it is independent of B3–B7 (it depends only on V2), so it can land while the multi-mode rows are still being written.

## What Changes

### 1. Scope (in and out)

**Implemented for real (Go binary answers these byte-exactly):**

| Invocation | stdout | stderr | exit |
|---|---|---|---|
| `tu help`, `tu -h`, `tu --help` (first positional; `tu help --dry-run` too) | `FullHelp` + `\n` (2,914 bytes) | empty | 0 |
| `tu help-dump` | the JSON envelope (§3; 3,224 bytes at v0.11.5) | empty | 0 |
| `tu skill [anything]` | `docs/site/skill.md` byte-for-byte (6,285 bytes, 117 lines) | empty | 0 |
| `tu shell-init bash` / `zsh` / `fish` | the embedded script for that shell | empty | 0 |
| `tu shell-init` | empty | the 6-line usage block (§5) | 2 |
| `tu shell-init tcsh` | empty | `Unknown shell: tcsh. Supported: bash, zsh, fish` | 2 |
| `tu update --help` / `tu update -h` (flag anywhere after `update`) | `FullHelp` + `\n` | empty | 0 |
| `tu update` off-Homebrew (executable does not resolve under `/Cellar/tu/`) | two lines (§4) | empty | 0 |
| `tu update [--skip-brew-update]` on Homebrew | the wrapper lines (§4) around the brew calls | one `Error: …` line on a brew failure | 0 / 1 |

**Not touched (stays exactly as it is):** every data command, `sync`, `--watch`, `lb`/`lbh`, multi mode, `-u`, `--by-machine`, `--top` (their placeholder rows stand); `src/node/` (D4 freeze — the TS is the oracle, not a patient); `docs/specs/*`, `README.md`, `docs/site/*`, `harness/matrix.json`, `.github/workflows/*`, `Formula/`, the plan document (the operator owns row status). `scripts/build.sh` is not edited: the Node `--define` embed and its `cmp` guard keep guarding the shipped bundle.

**Deliberately NOT changed although the audit found them** (§8, Open Questions): `tu skill topics` printing the bundle instead of an empty stdout; `-v` as an undocumented version alias (DC-09, already on the G0 list).

### 2. Harness gate

Baseline on this worktree (2026-09-17, `just go-diff --placeholder`): **134 green / 234 red / 0 timeout** of 368. The ten B8 cases are all red today with `exit: node=0|2 go=1` (the placeholder). After B8 they are green and the summary reads **144 green**:

```
help/single/default/pipe/fixed          help/single/default/tty/fixed
help-cmd/single/default/pipe/fixed      help-dump/single/default/pipe/fixed
skill/single/default/pipe/fixed         shell-init-bash/single/default/pipe/fixed
shell-init-zsh/single/default/pipe/fixed shell-init-fish/single/default/pipe/fixed
shell-init-missing/single/default/pipe/fixed shell-init-unknown/single/default/pipe/fixed
```

`version` and `version-short` are already green (P2) and must stay green after the `versionLine` move (§7). No other case may change colour. The matrix has no `update` group (it would spawn brew) and none is added — that is a G0/R3 matrix decision, not B8's.

Verification recipe for apply/review: `just go-diff --placeholder --filter help`, `--filter skill`, `--filter shell-init`, then the full run for the 144 count. The tty `help` case compares the `script(1)` transcript (CRLF endings; both sides identical by construction since both print the same bytes to a pty).

### 3. `internal/toolkit` — `help-dump` and the help text

**`FullHelp` lives in `internal/command`** beside `ShortUsage` (`request.go`) as the byte-exact TS `FULL_HELP` constant (a Go raw string; the text contains no backticks). It is grammar documentation, the same kind of constant as `ShortUsage`, and `command` already owns the grammar. `cmd/tu` prints it with `fmt.Fprintln(stdout, command.FullHelp)` — the TS `console.log(FULL_HELP)` appends exactly one `\n`.

**`toolkit/helpdump.go`** — the pure envelope, mirroring `help-dump.ts`:

```go
const (
    Tool              = "tu"
    Description       = "AI coding assistant cost tracking CLI" // package.json "description"
    HelpSchemaVersion = 1
)

// HelpNode / HelpDoc field ORDER is the contract: tool, version, schema_version, root;
// name, path, short, usage, text, commands. Commands is never nil ("[]" for the leaf).
type HelpNode struct {
    Name     string     `json:"name"`
    Path     string     `json:"path"`
    Short    string     `json:"short"`
    Usage    string     `json:"usage"`
    Text     string     `json:"text"`
    Commands []HelpNode `json:"commands"`
}
type HelpDoc struct {
    Tool          string   `json:"tool"`
    Version       string   `json:"version"`        // BARE: "0.11.5", never "v0.11.5"
    SchemaVersion int      `json:"schema_version"`
    Root          HelpNode `json:"root"`
}

// BuildHelpDoc assembles the flat document: one root node, Commands []HelpNode{}.
// text = helpText verbatim (the caller passes FullHelp + "\n"); usage = the first line
// starting "Usage:" (fallback: first non-empty line); short = Description.
func BuildHelpDoc(version, helpText string) HelpDoc

// Encode writes the pretty JSON exactly as JSON.stringify(doc, null, 2) + "\n" does:
// json.NewEncoder(w) with SetEscapeHTML(false) and SetIndent("", "  "); Encode appends "\n".
func (d HelpDoc) Encode(w io.Writer) error
```

Byte-exactness notes (all verified against the node capture): the version is bare (`"version": "0.11.5"`) while the Go binary is stamped `v0.11.5` — hence `BareVersion` (§7); `SetEscapeHTML(false)` is load-bearing (`<date>` etc. must stay raw — the default encoder would emit `<date>`); `MarshalIndent`/`SetIndent` renders an empty slice as `"commands": []` on one line, matching JS; the em dashes in the help text are raw UTF-8 on both sides; the document ends with `}` + `\n`. No `captured_at`, no `aliases` key (the standard says omit the key entirely when there are no aliases). `help-dump` stays out of `FullHelp` and out of every completion script (hidden).

`Description` is a Go constant (the long-term shape — `package.json` goes away at Z1). A transitional guard test reads `package.json` by walking up from the package directory (the `defaults_test.go` walk) and asserts `description` equals `Description`, skipping only when no `package.json` is found.

### 4. `internal/toolkit` — `update`

The TS `runUpdate` sequence, byte-exact wrapper lines, reproduced through an injected brew driver so `toolkit` never writes to stdout/stderr itself (the `internal/sync` `Exec` precedent: `metricsync.Exec{}.Clone(ctx, url, dir, stdin, stdout, stderr)` passes the process streams through, and `cmd/tu` decides what to print).

```go
// Brew is the driver seam; BrewExec is the real one, tests use a fake.
type Brew interface {
    Update(ctx context.Context) error                      // `brew update --quiet`, streams captured
    Info(ctx context.Context) ([]byte, error)              // `brew info --json=v2 tu`, stdout returned
    Upgrade(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error // `brew upgrade tu`, streams inherited
}
type BrewExec struct{}

func IsBrewInstall(resolvedExe string) bool               // strings.Contains(resolvedExe, "/Cellar/tu/")
func NotBrewInstallLines(version string) []string         // the two off-Homebrew lines below
func CheckLatest(ctx context.Context, brew Brew, skipBrewUpdate bool) (latest string, err *UpdateError)
func UpToDate(version, latest string) bool                // BareVersion(version) == latest
func Upgrade(ctx context.Context, brew Brew, stdin io.Reader, stdout, stderr io.Writer) *UpdateError

type UpdateError struct{ Message string }                 // the exact stderr line; exit 1 at the edge
func CurrentVersionLine(version string) string            // "Current version: v0.11.5"
func AlreadyUpToDateLine(version string) string           // "Already up to date (v0.11.5)."
func UpdatingLine(version, latest string) string          // "Updating v0.11.5 → v0.11.6..."
func UpdatedLine(latest string) string                    // "Updated to v0.11.6."
```

The edge (`cmd/tu` `runUpdate`) sequences it exactly as the TS does:

1. `--help` or `-h` anywhere in `req.Args` → print `FullHelp`, exit 0, run nothing (the `update` standard's flag-discovery probe: the help contains the literal `--skip-brew-update`). `Parse` already lands a non-leading `--help`/`-h` in the positionals, so both reach `Args`.
2. Gate: `os.Executable()` → `filepath.EvalSymlinks` → `IsBrewInstall`. Off-Homebrew (every dev build, the `go test` binary, the R2 dogfood install under `~/.local/bin`): print
   ```
   tu v0.11.5 was not installed via Homebrew.
   Update manually, or reinstall with: brew install sahil87/tap/tu
   ```
   exit 0 (verified against `node dist/tu.mjs update` on this machine). The gate string is `/Cellar/tu/` (the TS `__cli_dirname.includes("/Cellar/tu/")` and the spec), narrower than the siblings' `/Cellar/`.
3. `Current version: v0.11.5`.
4. Unless `req.Flags.SkipBrewUpdate` (the flag is a stripped boolean, so "anywhere on the command line" holds as in the TS): `brew.Update` — on any error, including `brew` missing from PATH: stderr `Error: could not check for updates (brew update failed). Check your network connection.`, exit 1.
5. `brew.Info` → parse `formulae[0].versions.stable`; a missing/empty/non-string value or any exec error → stderr `Error: could not determine latest version.`, exit 1.
6. `UpToDate` → `Already up to date (v0.11.5).`, exit 0.
7. `Updating v0.11.5 → v0.11.6...` then `brew.Upgrade` with `os.Stdin`/stdout/stderr passed through; error → stderr `Error: brew upgrade failed.`, exit 1; success → `Updated to v0.11.6.`, exit 0.

`BrewExec` argv is the TS argv **verbatim**: `brew update --quiet`, `brew info --json=v2 tu`, `brew upgrade tu` (not the siblings' fully-qualified `sahil87/tap/tu` — parity with the shipped behaviour; the fully-qualified form is `shll update`'s own compose, see Assumptions). Brew-handling safety, per the `update` standard and the TS posture:

- `Update` and `Info` are **bounded and graceful**: `exec.CommandContext` with `context.WithTimeout` of **600 s** and **60 s** respectively (the TS `timeout: 600_000` / `60_000`), `cmd.Cancel` overridden to send `SIGTERM` and `cmd.WaitDelay = 10 * time.Second` (the `wt` `newBoundedBrewCmd` helper, copied) — never the default `SIGKILL`.
- `Upgrade` has **no deadline and no `Cancel`**: `exec.CommandContext(context.Background(), …)` semantics, streams inherited, `cmd.Env = append(os.Environ(), "HOMEBREW_NO_ASK=1")` (Homebrew 6 ask-mode suppression — the env var, not `--no-ask`, so Homebrew < 6 is unaffected). Ctrl-C is the user's escape hatch. A unit test pins `Env` contains `HOMEBREW_NO_ASK=1`, `Args == [brew upgrade tu]`, `Cancel == nil`, `WaitDelay == 0` on the upgrade command and `Cancel != nil`, `WaitDelay == 10s` on the bounded ones.

Positional and data flags on `update` are ignored (DC-02 for setup commands holds here too: `Parse` sets `Command` regardless).

### 5. `internal/toolkit` — `shell-init` and the completion scripts

The three scripts are **embedded from plain files**, not transcribed into Go strings:

```
src/go/internal/toolkit/completions/tu.bash
src/go/internal/toolkit/completions/tu.zsh
src/go/internal/toolkit/completions/tu.fish
```

each the exact bytes the TS constant evaluates to (`BASH_COMPLETION`, `ZSH_COMPLETION`, `FISH_COMPLETION` — un-escaped: `\${` → `${`, `\\` → `\`, `` \` `` → `` ` ``; each ends with exactly one `\n`). Files sidestep the zsh script's backticks (which a Go raw string cannot hold) and the `${…}` forms, and `diff` against the node capture is the review check.

```go
//go:embed completions/tu.bash completions/tu.zsh completions/tu.fish
var completions embed.FS

const Shells = "bash, zsh, fish"        // the list in the unknown-shell message
func Completion(shell string) ([]byte, bool) // "bash" | "zsh" | "fish"; false otherwise
func UnknownShellMessage(shell string) string // "Unknown shell: tcsh. Supported: bash, zsh, fish"
const ShellInitUsage = `Usage: tu shell-init <bash|zsh|fish>

Install:
  bash: echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc
  zsh:  echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc
  fish: tu shell-init fish > ~/.config/fish/completions/tu.fish`
```

Edge behaviour (`cmd/tu`): `len(req.Args) == 0` → `Fprintln(stderr, ShellInitUsage)`, stdout empty, exit 2 (the node stderr capture is 223 bytes, i.e. the block plus `\n`); unknown shell → `Fprintln(stderr, UnknownShellMessage(shell))`, exit 2; known → `stdout.Write(script)`, no added newline, exit 0. Extra arguments after the shell are ignored (TS reads `filteredArgs[1]` only).

Standard-mandated test: each script is fed to its shell's parser — `bash -n`, `zsh -n`, `fish --no-execute` — in a subtest that `t.Skip`s when that shell is not on PATH (verified on this machine: `bash -n` and `zsh -n` accept the TS scripts). Plus a content test pinning the frozen token inventory the spec lists (non-data subcommands without `help-dump`; `w weekly wh lb lbh kimi ki gem cop`; every long flag including `--skip-brew-update`; every short flag; `cost tokens`; `bash zsh fish`) and the `# tu(1) <shell> completion` first line of each.

### 6. `internal/toolkit` — `skill` via `go:embed` + drift guards

The Go module root is `src/go/`, so `//go:embed` cannot reach `docs/site/skill.md`. B8 copies the sibling mechanism (`idea`/`hop`), which is also the repo's own `tu.default.conf` mechanism (`internal/config/defaults.go` + `defaults_test.go`):

- **Committed copy** `src/go/internal/toolkit/skill.md`, byte-identical to `docs/site/skill.md`.
- **`scripts/sync-skill.sh`** — `cp -f docs/site/skill.md src/go/internal/toolkit/skill.md` from the repo root, printing `synced skill bundle: src/go/internal/toolkit/skill.md`; a `//go:generate ../../../../scripts/sync-skill.sh` directive on `skill.go` documents the path.
- **`skill.go`**: `//go:embed skill.md` into `var Skill []byte`; `func WriteSkill(w io.Writer) error` writes it verbatim.
- **Test drift guard** (`skill_test.go`): walk up to the directory containing `package.json`, read `docs/site/skill.md`, assert byte-equality with `Skill`; assert ≤ 150 lines (the `skill` standard's hard budget, currently 117); assert the last byte is `\n`; assert `WriteSkill` output equals `Skill`.
- **Build drift guard** (`justfile` `go-build`): before `go build`, `cmp -s docs/site/skill.md src/go/internal/toolkit/skill.md || { echo "error: src/go/internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh" >&2; exit 1; }`. This is the "build drift guard" the row names, mirroring `scripts/build.sh`'s post-build `cmp` for the Node bundle, and it runs in CI's `go-build-and-test` lane (`Build` step) ahead of `go test`. `bin/tu` therefore can never be built from a stale copy.

Edge behaviour: `tu skill` (any args) → `WriteSkill(stdout)`, stderr empty, exit 0. The TS ignores arguments, so `tu skill topics` prints the bundle — reproduced as-is (Open Questions).

### 7. `--version` line and version normalization

```go
func BareVersion(v string) string     // strings.TrimPrefix(v, "v")           "v0.11.5" → "0.11.5"; "dev" → "dev"
func DisplayVersion(v string) string  // "v" + bare when bare starts with a digit; else as-is   → "v0.11.5"; "dev"
func VersionLine(v string) string     // "tu version " + DisplayVersion(v)
```

`versionLine` moves from `cmd/tu/main.go` to `toolkit.VersionLine` (the architecture table puts `--version` in `toolkit`); `var version` and the `-X main.version=` stamp stay in `main`, and `TestVersionLine` moves with the function (the `version`/`version-short` harness cases must stay green). `help-dump` uses `BareVersion`; the `update` lines use `DisplayVersion` (the TS prints `v${PKG_VERSION}` from a bare package.json version, which is the same as `DisplayVersion` for every stamped build; an unstamped `dev` build prints `tu dev was not installed via Homebrew.`, which no shipped or harness path reaches).

### 8. Standards audit — shll v0.1.32 (2026-09-17)

Each surface was checked clause by clause against the standard as installed (`shll version v0.1.32`; standards read with `shll standards <name>`). Result per surface, for the Go implementation as specified above:

| Standard | Clauses | Go result | Notes |
|---|---|---|---|
| `help-dump` | stdout-only JSON, stderr empty, exit 0; hidden and self-filtering; envelope `{tool, version, schema_version, root}`; no `captured_at`; `version` from the built binary; `aliases` omitted when none; walk-never-parse; pinning test | conformant | tu keeps the flat document (`commands: []`, full `--help` in `root.text`) the standard's "tu exception" describes; the version comes from `-ldflags`, bare. The standard's prose still says "Node/TS" — X4's job |
| `update` | in-place upgrade; standalone; prompt-free incl. wrapped brew; `--skip-brew-update` literal in `update --help` and honored; exit 0 on success incl. up-to-date, non-zero only on failure; no `SIGKILL`, no short hard timeout on `brew upgrade`, graceful bounds; `/Cellar/` gate with a clear off-brew message; one-name-four-places; `v{semver}` tags | conformant | `HOMEBREW_NO_ASK=1` satisfies the wrapped-subprocess prompt-free MUST; bounds only on the metadata calls, SIGTERM + 10 s grace; `brew upgrade` unbounded |
| `shell-init` | eval-safe stdout, exit 0 for the named shell; diagnostics stderr only; non-zero on any failure; missing/unsupported shell → exit 2, usage on stderr, stdout empty; eval/parse test | conformant | three shells; the parse test is the standard's recommended guard |
| `skill` | exact name `skill`; raw markdown stdout, stderr empty, exit 0; static only; ≤ 150 lines; byte-identical to `docs/site/skill.md` via sync + drift guard; budget and topics contract pinned by failing tests; no content topic named `topics` | conformant except one | **Finding S1**: the reserved `skill topics` clause requires `tu skill topics` to print empty stdout, exit 0; the shipped TS prints the whole bundle (arguments are ignored). The clause post-dates tu's adoption audit (shll v0.0.23, 2026-07-18). B8 reproduces the TS (frozen surface). See Open Questions |
| `version` | `--version` exit 0 to stdout; < 2 s, no network; token on the first line; canonical `<tool> version vX.Y.Z`; binary name == tool name; pinning test | conformant | already true since P2; `VersionLine` keeps the shape. **Finding V1 (pre-existing, DC-09)**: `-v` is an extra alias the standard does not require and `--help` does not list — on the G0 list already |

The audit record (version, date, per-surface result, the two findings) is hydrated into the new `go-port/toolkit-layer` memory and appended to the `build/toolchain` audited-standards ledger. The spec's § Toolkit Contracts already names v0.1.32 and needs no edit.

### 9. `cmd/tu` — dispatch and write order

`run()` is unchanged up to `runCommand`. `runCommand` grows from three real commands to eight; the placeholder `default` shrinks to `sync` (B6). None of the new commands consult `$HOME` (the spec: `help`, `--version`, `help-dump`, `skill`, `shell-init`, `update` work with `$HOME` unset), so they are answered **before** `config.ResolvePaths`, exactly where the placeholder branch sits today:

```go
switch req.Command {
case "help", "-h", "--help":   fmt.Fprintln(stdout, command.FullHelp); return ExitOK
case "help-dump":              toolkit.BuildHelpDoc(toolkit.BareVersion(version), command.FullHelp+"\n").Encode(stdout); return ExitOK
case "skill":                  toolkit.WriteSkill(stdout); return ExitOK
case "shell-init":             return runShellInit(req.Args, stdout, stderr)
case "update":                 return runUpdate(req, stdout, stderr)   // §4 sequence, toolkit.BrewExec{}, os.Stdin
case "init-conf", "init-metrics", "status": // as today
default:                       placeholder (sync)
}
```

`TestRunNotImplemented` currently uses `--help` as its unported example and must switch to a still-unported argv (e.g. `sync`); `cmd/tu/main_test.go` gains table-driven cases for every row of §1's table that needs no brew (help ×3 plus `help --dry-run`, help-dump parse + bare version + `<` unescaped + stderr empty, skill == `toolkit.Skill`, the five `shell-init` cases, `update --help`/`-h`, and `update` off-Homebrew — the test binary never lives under `/Cellar/tu/`, so its two lines and exit 0 are deterministic). The Homebrew path is unit-tested in `toolkit/update_test.go` with a fake `Brew` recording calls (skip flag skips `Update`; each error maps to its line; up-to-date short-circuits before `Upgrade`; upgrade streams reach the passed writers).

Byte references from the 2026-09-17 node captures (`bin/harness/report/cases/<id>/single/default/pipe/fixed/node.*`):

- `help-cmd`: stdout 2,914 B, stderr 0 B, exit 0 — `FULL_HELP` + `\n`.
- `help-dump`: stdout 3,224 B, exit 0; keys `tool,version,schema_version,root` / `name,path,short,usage,text,commands`; `"version": "0.11.5"`; `"short": "AI coding assistant cost tracking CLI"`; `"usage": "Usage: tu [source] [period] [display]"`; `text` ends with `\n`; `<date>` unescaped; `"commands": []`.
- `skill`: stdout 6,285 B == `docs/site/skill.md` (`cmp` clean).
- `shell-init-missing`: stdout 0 B, stderr 223 B (the §5 block + `\n`), exit 2.
- `shell-init-unknown`: stderr `Unknown shell: tcsh. Supported: bash, zsh, fish\n`, exit 2.
- `help` (tty): transcript ends `…in watch mode\r\n`.
- `update` (this machine, off-Homebrew): the two §4 lines, exit 0; `update --help` first line `Usage: tu [source] [period] [display]`, exit 0, contains `--skip-brew-update` once.

### 10. Tests summary

- `internal/toolkit`: `helpdump_test.go` (envelope bytes for a fixed help text and version — a small golden; bare version; no HTML escaping; trailing newline; `Description` vs `package.json` transitional guard), `version_test.go` (table: `v0.11.5`, `0.11.5`, `dev`), `shellinit_test.go` (per-shell parse via `bash -n`/`zsh -n`/`fish --no-execute` with `t.Skip` when absent; first-line and token-inventory content checks; missing/unknown messages), `skill_test.go` (drift guard, ≤ 150 lines, trailing `\n`, `WriteSkill` identity), `update_test.go` (fake `Brew`; the bounded/unbounded command shapes and the `HOMEBREW_NO_ASK=1` env).
- `cmd/tu/main_test.go`: the §9 table; `TestVersionLine` follows the move.
- Harness: the ten §2 cases green; `just go-diff --placeholder` summary `144 green`.
- `just go-lint`, `just go-test`, `just go-build` (now with the skill `cmp` guard) clean.

## Affected Memory

- `go-port/toolkit-layer`: (new) the `internal/toolkit` package — `FullHelp`'s home in `command`, `BuildHelpDoc`/`Encode` and the `SetEscapeHTML(false)` rule, `BareVersion`/`DisplayVersion`/`VersionLine`, the `Brew` driver seam with `BrewExec`'s verbatim argv, bounds (600 s / 60 s, SIGTERM + 10 s grace) and the unbounded `HOMEBREW_NO_ASK=1` upgrade, `IsBrewInstall`'s `/Cellar/tu/` gate and the off-Homebrew lines, the embedded completion files and `ShellInitUsage`/`UnknownShellMessage`, the embedded `skill.md` copy with `scripts/sync-skill.sh` and both drift guards, the `cmd/tu` sequences and exit codes for each command; the **standards audit record** (shll v0.1.32, 2026-09-17, per-surface results, findings S1 and V1). Design Decisions: driver-injected `update` (nothing prints below `cmd/tu`); files not strings for the completions; committed copy + sync + two guards for the skill; TS argv verbatim over the siblings' fully-qualified formula.
- `go-port/command-edge`: (modify) `runCommand`'s eight real commands and the `sync`-only placeholder; the row map (B8's surfaces now implemented); `FullHelp` beside `ShortUsage`; the `versionLine` move; the harness status line (144/368).
- `build/toolchain`: (modify) `just go-build`'s skill drift guard and `scripts/sync-skill.sh`; the `cmd/tu` bullet's implemented-surface list and the `go-diff` count; the audited-standards ledger gains the Go audit against **shll v0.1.32** (2026-09-17, this change) with findings S1/V1.

`cli/data-pipeline`, `configuration/*`, `harness/differential-harness` are not modified: no TS, spec, or harness code changes.

## Impact

- **New Go code**: `internal/toolkit` (~350 lines across `helpdump.go`, `version.go`, `shellinit.go`, `skill.go`, `update.go`) plus three embedded completion files (~230 lines, copied bytes), the embedded `skill.md` copy (117 lines), `scripts/sync-skill.sh` (~15 lines); `internal/command/request.go` +~55 lines (`FullHelp`); `cmd/tu/main.go` +~90 lines (`runShellInit`, `runUpdate`, the dispatch rows), −10 (`versionLine`); `justfile` +4. Tests ~450–550 lines. Solidly **M**.
- **Dependencies**: none new (`embed`, `encoding/json`, `os/exec`, `syscall` are stdlib). `go.mod` unchanged.
- **CI**: no workflow edits; `just go-build` gains the `cmp` step the `Build` step already runs. A stale `skill.md` copy now fails the Go lane at build, not at test.
- **Harness**: +10 green (134 → 144). `go-diff` stays informational.
- **Downstream rows**: R1 packages this binary as the dogfood asset (`update`'s off-Homebrew message is exactly what R2's `~/.local/bin/tu` install should print — the plan already says so); X4 rewrites the standards' "tu exception" prose once the Go binary ships (this intake confirms the flat document stays); B6 inherits the last placeholder row in `runCommand`.
- **Risk**: transcription. The completion scripts and `FullHelp` are long verbatim strings — the harness catches any slip byte-for-byte, and embedding the scripts as files removes the escaping step entirely. The one silent-divergence trap (`encoding/json` HTML escaping) is pinned by a unit test that asserts `<date>` appears raw in the encoded output.

## Open Questions

- **S1 — `tu skill topics` (standards finding, frozen surface).** shll v0.1.32's `skill` standard reserves the topic name `topics` and requires `<tool> skill topics` to print empty stdout, exit 0, for every adopting tool. tu's TS ignores arguments and prints the bundle (`node dist/tu.mjs skill topics` → the bundle, exit 0), so the Go port reproduces that. The fix is a behaviour change on a Goal-frozen surface: either a D4-sanctioned TS bug fix ported in the same window (plus a matrix case), or a Go-only change after cutover. Not B8's call — flagged for Sahil / the G0 list. Note the spec's line "tu ships no `skill topics` pages" is true but does not describe the current `tu skill topics` output.
- **V1 — `-v` alias (DC-09, already on the G0 list).** Restated here only because the `version` audit re-touched it; B8 keeps `-v`.
- **`update` bounds posture.** B8 keeps the TS bounds on the two metadata calls (600 s / 60 s, now SIGTERM-graceful). The siblings `idea` and `hop` run every brew call unbounded. Either is standard-conformant; if G-review or X-phase prefers uniformity with the siblings, dropping the bounds is a two-line change in `BrewExec`.
- **Formula name in brew argv.** TS calls `brew info --json=v2 tu` / `brew upgrade tu`; siblings use `sahil87/tap/<tool>`. B8 keeps the TS argv (parity). If a same-named core formula ever appears, the fully-qualified form is the fix — post-cutover.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope = `help`/`-h`/`--help`, `help-dump`, `skill`, `shell-init` (3 shells + 2 usage errors), `update` (help, gate, skip flag, brew sequence); everything else keeps the placeholder | Row text; the command-edge row map assigns exactly these to B8; the matrix's 10 toolkit cases | S:90 R:85 A:95 D:90 |
| 2 | Certain | Harness gate = the 10 case IDs in §2, summary 144 green; `version`/`version-short` stay green | Enumerated with `tudiff run --list`; baseline 134 measured on this worktree today | S:90 R:90 A:95 D:90 |
| 3 | Certain | New leaf package `internal/toolkit`; `cmd/tu` remains the only writer; `versionLine` moves to `toolkit.VersionLine`, `var version` stays in `main` | Target architecture table names `toolkit` with this exact responsibility list; G1 checklist item 1/4; `-X main.version` stamp is a justfile contract | S:85 R:80 A:90 D:85 |
| 4 | Confident | `FullHelp` lives in `internal/command` beside `ShortUsage`; `toolkit.BuildHelpDoc(version, helpText)` takes the text as a parameter | `ShortUsage` precedent; keeps `toolkit` a leaf and mirrors the TS `buildHelpDoc({helpText})` input shape | S:60 R:90 A:85 D:70 |
| 5 | Certain | help-dump encoding via `json.Encoder` with `SetEscapeHTML(false)` + `SetIndent("", "  ")`; version bare via `BareVersion`; `Commands` non-nil; `Description` constant guarded against `package.json` | Node capture shows `<date>` raw and `"version": "0.11.5"`; Go's default encoder would emit `<`; MarshalIndent renders `[]` inline | S:85 R:90 A:95 D:90 |
| 6 | Certain | Completion scripts embedded from three files with the TS escapes resolved; `ShellInitUsage` and the unknown-shell message byte-exact; missing/unknown → exit 2, stdout empty | The zsh script contains backticks a Go raw string cannot hold; harness `shell-init-*` cases pin the bytes; spec § shell-init | S:90 R:90 A:95 D:90 |
| 7 | Confident | Standard's eval/parse test implemented as `bash -n` / `zsh -n` / `fish --no-execute` subtests that skip when the shell is absent | The `shell-init` standard recommends it; verified `bash -n`/`zsh -n` accept the TS scripts here; skipping keeps CI portable | S:65 R:90 A:85 D:75 |
| 8 | Certain | Skill: committed copy `src/go/internal/toolkit/skill.md` + `scripts/sync-skill.sh` + test drift guard + `just go-build` `cmp` guard | Row text ("go:embed plus a build drift guard"); sibling `sync-skill.sh` pattern (D5); the repo's own `tu.default.conf` embed precedent; module root blocks a direct embed | S:85 R:85 A:95 D:85 |
| 9 | Certain | `tu skill <args>` prints the bundle for any arguments, including `topics` | TS ignores args (verified); Goal freezes the surface; the standards conflict is recorded as S1, not fixed here | S:80 R:90 A:95 D:85 |
| 10 | Certain | `update` driven through a `Brew` interface (`BrewExec` real); `toolkit` writes nothing to stdout/stderr; `cmd/tu` sequences and prints; upgrade streams passed through | `internal/sync` `Exec` + `CloneStep` precedent; G1 items 1 and 4; testability without brew | S:75 R:80 A:90 D:80 |
| 11 | Certain | `update` wrapper lines and error messages byte-exact to `runUpdate` (§4), exit 0 for off-Homebrew, up-to-date and `--help`; exit 1 for the three brew failures | Read from `cli.ts`; off-Homebrew output verified live; spec § update and § Exit Codes | S:90 R:90 A:95 D:95 |
| 12 | Certain | Brew gate string is `/Cellar/tu/` on the symlink-resolved `os.Executable()` | TS `includes("/Cellar/tu/")`; spec wording; siblings' `EvalSymlinks` mechanism | S:85 R:90 A:95 D:90 |
| 13 | Confident | Brew argv verbatim from the TS: `brew update --quiet`, `brew info --json=v2 tu`, `brew upgrade tu` (not `sahil87/tap/tu`) | Parity with months of shipped behaviour; the fully-qualified form is `shll update`'s compose; no core `tu` formula exists | S:70 R:90 A:80 D:70 |
| 14 | Confident | `brew update`/`brew info` bounded at 600 s / 60 s with `Cancel` → `SIGTERM` and `WaitDelay` 10 s; `brew upgrade` unbounded, `HOMEBREW_NO_ASK=1` appended to `os.Environ()` | TS bounds and the memory's documented posture; `update` standard MUST/SHOULD; `wt` helper copied; siblings differ (unbounded) — reversible in one function | S:70 R:90 A:85 D:65 |
| 15 | Certain | `update --help`/`-h` detected anywhere in `req.Args`, prints `FullHelp`, exit 0, runs nothing | TS `rawArgs.includes`; `Parse` lands non-leading `--help`/`-h` in positionals → `Args`; the standard's substring probe | S:85 R:90 A:95 D:90 |
| 16 | Certain | `DisplayVersion` prefixes `v` only for digit-leading versions; unstamped `dev` prints `dev` | Existing `versionLine` rule; TS never sees a non-numeric version; no shipped path reaches it | S:70 R:95 A:90 D:85 |
| 17 | Certain | New commands answered before `config.ResolvePaths` (no `$HOME` needed) | Spec § Non-data commands `$HOME` dependence; today's placeholder branch already precedes the HOME check | S:85 R:90 A:95 D:95 |
| 18 | Certain | Standards audited: `help-dump`, `update`, `shell-init`, `skill`, `version` against shll **v0.1.32** on 2026-09-17; record goes to memory (`go-port/toolkit-layer`, `build/toolchain` ledger), no spec edit | Row text; `shll version` output; spec already cites v0.1.32; the two findings are frozen-surface items | S:90 R:90 A:95 D:90 |
| 19 | Certain | Tests: table-driven `cmd/tu` cases for every brew-free row, fake-`Brew` unit tests, a help-dump golden, drift guards, `TestRunNotImplemented` re-pointed at `sync` | G1 item 5; `main_test.go` conventions; `--help` stops being unported | S:80 R:90 A:90 D:85 |
| 20 | Confident | Memory: new `go-port/toolkit-layer`, modify `go-port/command-edge` and `build/toolchain`; TS memories untouched | One file per Go layer (V2/B1/B2 convention); the audit ledger already lives in `build/toolchain` | S:55 R:90 A:80 D:70 |
| 21 | Certain | No external surface changes: no `src/node`, spec, `docs/site`, README, harness, CI workflow, formula or plan-doc edits; Go stays unshipped | Row text and plan Goal; D2/D4; the operator owns row status | S:95 R:90 A:95 D:95 |

21 assumptions (17 certain, 4 confident, 0 tentative, 0 unresolved).
