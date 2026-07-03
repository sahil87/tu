# Intake: ccusage v20 Native Migration

**Change**: 260703-bxuh-ccusage-v20-native-migration
**Created**: 2026-07-03

## Origin

> Can you check if the ccusage dependency used in this repo is up to date?

Conversational — asked during a `/fab-discuss` session. Research established that all three ccusage packages are stale (locked at 18.0.11; latest is `ccusage@20.0.14` and `@ccusage/codex`/`@ccusage/opencode` are **deprecated with 19.0.0 as their final release**). The user was presented three paths (v19.0.3 JS stopgap, straight to v20 native, stay on 18.x) and **explicitly chose "Straight to v20 native"**, running it as a fab change.

Key facts established during the conversation (verified against npm tarballs and the live tap formula, not assumed):

- `ccusage@20` ships **no JS implementation** — the main package is a 5-file launcher (`src/cli.js`) that `require.resolve`s a per-platform native package and spawns its Rust binary. There is **no JS fallback**: if the native package is absent the launcher exits 1 with an error. `dist/*.js` no longer exists, so tu's current vendoring (`cp node_modules/ccusage/dist/*.js`) is impossible on v20.
- Native binaries live in `@ccusage/ccusage-{darwin,linux,win32}-{x64,arm64}` npm packages (declared as `optionalDependencies` of `ccusage`), each containing `bin/ccusage` (`bin/ccusage.exe` on win32).
- v19 made ccusage the all-agent CLI: `ccusage codex daily` and `ccusage opencode daily` are built-in subcommands, replacing the deprecated standalone packages.
- `ccusage@20` has **no `engines` constraint** in package.json (the Rust binary does the work).
- Upstream states v19→v20 "command behavior and output remain compatible"; v19 also carried correctness fixes tu benefits from (dedupe of Claude entries without request IDs, dedupe of copied Codex token events) and large performance gains.
- **The Homebrew formula builds from source on the user's machine**: `Formula/tu.rb` fetches the git tag, runs `npm install --include=dev` + `npm run build`, then `libexec.install "dist/tu.mjs"` and `libexec.install "dist/vendor"`. There is no prebuilt bottle. This means npm's `optionalDependencies` mechanism installs the **correct platform binary automatically at install time** — no per-platform release artifacts and no `release.yml` changes are needed.

## Why

1. **Pain point**: tu vendors ccusage 18.0.11, a deprecated line. New Claude/Codex/OpenCode log formats, new models, and parser fixes land only on v20. Concretely, v19 fixed double-counting of copied Codex token events and dedupe of Claude entries lacking request IDs — cost-correctness bugs tu currently ships. `tu watch` also re-invokes ccusage repeatedly, so the v19/v20 performance work ("up to 100x faster" JS optimization, then a native Rust core) directly improves tu's hot path.
2. **Consequence of inaction**: growing drift on a dead line — cost numbers silently become wrong as upstream tools evolve their log formats, and no dependabot bump can ever fix it (a naive `npm update` to v20 breaks `scripts/build.sh` outright because `dist/*.js` no longer exists).
3. **Why this approach**: the v19.0.3 JS stopgap was considered and rejected by the user — it lands on an equally dead line (superseded within two days of release) and forces a second migration later. Going straight to v20 is viable at small scope because the formula's source-build distribution makes platform selection automatic (see Origin facts). The fetcher/build changes are the same ones v19 would have needed.

## What Changes

### package.json — dependency consolidation

- Remove `@ccusage/codex` and `@ccusage/opencode` (deprecated upstream; final release 19.0.0).
- Bump `ccusage` from `^18.0.8` to `^20.0.14` (caret + committed lockfile keeps installs reproducible; dependabot continues to bump within-major).
- `engines.node >= 18` **unchanged** — vendor mode executes the Rust binary directly (no Node involvement), and dev mode's JS launcher is trivial.

### scripts/build.sh — vendor the host-platform native binary

Replace the three `cp node_modules/*/dist/*.js` blocks with a single native-binary vendoring step:

```bash
rm -rf dist/vendor
# Map host platform to the ccusage native package name
PLATFORM_PKG=$(node -p '`@ccusage/ccusage-${process.platform}-${process.arch}`')
BIN_SRC="node_modules/${PLATFORM_PKG}/bin/ccusage"
if [ ! -f "$BIN_SRC" ]; then
  echo "error: ${PLATFORM_PKG} not installed — unsupported platform or npm install skipped optional deps" >&2
  exit 1
fi
mkdir -p dist/vendor/ccusage/bin
cp "$BIN_SRC" dist/vendor/ccusage/bin/ccusage
chmod 0755 dist/vendor/ccusage/bin/ccusage
```

Fail-loud is correct here (build-time, not runtime — Constitution II applies to runtime data sources). Supported platforms are exactly Homebrew's: darwin/linux × x64/arm64. win32 is out of scope for the vendor path (brew-only distribution); Windows dev mode still works via the npm launcher.

### src/node/core/fetcher.ts — single binary, subcommand grammar

`TOOLS` currently points at three binaries (vendor mode: `node {BIN}/{pkg}/index.js`; dev mode: `{BIN}/{name}` from `node_modules/.bin`). New shape — one binary, per-tool subcommand prefixes, reusing the existing `prefixArgs` mechanism:

```ts
const CCUSAGE = useVendor ? `${BIN}/ccusage/bin/ccusage` : `${BIN}/ccusage`;

export const TOOLS: Record<string, ToolConfig> = {
  cc:    { name: "Claude Code", binary: CCUSAGE, prefixArgs: [],           needsFilter: false },
  codex: { name: "Codex",       binary: CCUSAGE, prefixArgs: ["codex"],    needsFilter: true },
  oc:    { name: "OpenCode",    binary: CCUSAGE, prefixArgs: ["opencode"], needsFilter: true },
};
```

- Vendor mode: exec the native binary directly — `binary` is the Rust executable, no `node` interpreter.
- Dev mode: `node_modules/.bin/ccusage` is v20's JS launcher, which resolves the host's optional native dep — same grammar, works unchanged.
- `runTool` composes `[...prefixArgs, period, "--json", ...extraArgs]` — e.g. `ccusage codex daily --json`. No change to `runTool` itself.
- `needsFilter`/`stripNoise` retained defensively for codex/opencode pending verification against real v20 output (drop later if provably unnecessary — not in this change).
- JSON parsing (`parsed["daily"]`, `date`/`month` labels, `totalCost|costUSD`, `cachedInputTokens` fallbacks) is expected compatible per upstream; **apply MUST verify against real local data** (this dev box has live `~/.claude` history) and adjust `toUsageTotals`/`parseJson` only if drift is observed.

### Tests

- Update `src/node/core/__tests__/fetcher.test.ts` (and any other tests referencing `TOOLS` shape, e.g. `fetch-warning.test.ts`) for the single-binary registry.
- Existing parser tests (`toUsageTotals`, `normalizeLabel`, `pickCurrentEntry`, `stripNoise`) stay as-is — the parse contract is unchanged.

### Out of scope / no changes needed

- **`.github/workflows/release.yml`** — untouched. The release is tag-anchored with no prebuilt platform artifacts; its `npm ci && npm run build` step will exercise the new build.sh on linux-x64 as a fail-loud check.
- **`sahil87/homebrew-tap` `Formula/tu.rb`** — expected to need **no change**: `npm install --include=dev` installs the platform's optional dep, `npm run build` vendors the binary, `libexec.install "dist/vendor"` ships it (Homebrew preserves file modes). Verification at ship time: after release, `brew install`/`brew upgrade` on macOS and confirm `tu` returns real data. If the exec bit is lost in transit, the fix is one `chmod 0755, libexec/"vendor/ccusage/bin/ccusage"` line in the formula — a manual cross-repo follow-up, not part of this PR.
- **`~/.tu/cache`** — no migration: cached payloads are already-parsed `UsageEntry[]`, shape unchanged.

## Affected Memory

- `build/toolchain`: (modify) Vendor-distribution section — three vendored JS packages become one vendored native Rust binary (`dist/vendor/ccusage/bin/ccusage`); dependency list drops `@ccusage/codex`/`@ccusage/opencode`; document the platform-package mapping and fail-loud build guard
- `cli/data-pipeline`: (modify) Tool registry — single `ccusage` binary with `codex`/`opencode` subcommand `prefixArgs`; vendor mode execs the native binary directly (no `node`), dev mode uses the npm launcher

## Impact

- **Code**: `package.json`, `package-lock.json`, `scripts/build.sh`, `src/node/core/fetcher.ts`, `src/node/core/__tests__/fetcher.test.ts` (+ sibling tests touching `TOOLS`)
- **Behavior**: none intended — same commands, same output, same cache. Cost numbers may shift slightly (correctly) due to upstream dedupe fixes.
- **Distribution**: dist/vendor shrinks from ~3 JS bundles to one native binary; brew install time and startup latency improve (Constitution IV)
- **Versioning**: minor bump (distribution-internal change, output format stable per Constitution — Output Stability)
- **Risk**: platform coverage (darwin/linux × x64/arm64 all published at 20.0.14 — verified on npm); JSON drift (mitigated by apply-time verification against real data)

## Open Questions

- None blocking. Ship-time verification item: confirm exec bit survives `libexec.install` on a real macOS brew install (fallback documented in What Changes).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Target `ccusage@^20.0.14`; drop `@ccusage/codex` + `@ccusage/opencode`; use built-in `codex`/`opencode` subcommands | User explicitly chose v20 over v19 stopgap; upstream deprecated the standalone packages (final release 19.0.0) | S:95 R:70 A:95 D:95 |
| 2 | Certain | Platform selection via npm `optionalDependencies` at brew source-build time; no per-platform release artifacts; `release.yml` untouched | Verified: `Formula/tu.rb` builds from the git tag with `npm install` on the user's machine — npm installs the host's native package automatically | S:85 R:75 A:90 D:85 |
| 3 | Certain | win32 excluded from the vendor path; darwin/linux × x64/arm64 only | Distribution is Homebrew-only; Windows dev mode still works via the npm JS launcher | S:75 R:90 A:90 D:85 |
| 4 | Confident | Vendor layout `dist/vendor/ccusage/bin/ccusage`, exec'd directly (no `node`) in vendor mode; dev mode uses `node_modules/.bin/ccusage` launcher | Mirrors upstream package layout (`bin/ccusage`); reuses existing vendor-first `BIN` resolution and `prefixArgs` mechanism unchanged | S:70 R:80 A:80 D:70 |
| 5 | Confident | `engines.node >= 18` floor unchanged | Vendor mode bypasses Node for ccusage entirely; `ccusage@20` itself declares no engines constraint (verified in tarball) | S:60 R:85 A:80 D:75 |
| 6 | Confident | v20 JSON output parses with the existing `toUsageTotals`/`parseJson` contract; apply verifies against real local `~/.claude` data before finishing | Upstream release notes state output compatibility across v19/v20; verification (not trust) is an apply task, and parser adjustment is in scope if drift is found | S:65 R:75 A:70 D:70 |
| 7 | Confident | Keep `needsFilter`/`stripNoise` defensively for codex/opencode | Harmless if v20 emits clean stdout; removing it is a separate cleanup once proven unnecessary | S:55 R:90 A:75 D:70 |
| 8 | Confident | Keep caret range `^20.0.14` (not exact pin); committed `package-lock.json` keeps brew + CI installs reproducible | Matches existing convention (`^18.0.8`) and the dependabot bump flow; `npm install` in the formula honors the lockfile from the tagged source | S:50 R:85 A:60 D:55 |
| 9 | Confident | Formula `tu.rb` needs no change; exec-bit survival verified at ship time with a documented one-line fallback | `libexec.install` preserves modes in normal operation; risk contained to a manual cross-repo one-liner if wrong | S:55 R:70 A:65 D:65 |
| 10 | Confident | `~/.tu/cache` requires no migration | Cache stores parsed `UsageEntry[]`, whose shape this change does not touch | S:60 R:85 A:85 D:75 |

10 assumptions (3 certain, 7 confident, 0 tentative, 0 unresolved).
