# Intake: Cutover formula — release.yml pushes the Go formula to the tap

**Change**: 260923-jmh4-cutover-formula
**Created**: 2026-09-23

## Origin

One-shot `/fab-new` from the plan's queue handoff (plan row X1, Phase 4). Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row X1. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Scope: wire release.yml so that on a minor version bump (the cutover release, 0.12.0) it pushes the generated Go formula (dist/tu.rb, built by R1's package-go.sh / go-formula.sh) to the sahil87/homebrew-tap repo instead of the Node-based formula — prebuilt tarballs, no depends_on node. Node build steps stay in CI (still needed by the harness) but no longer feed the pushed formula. README install prose is unchanged (Policy B: it points at hexokit.com). This change wires the mechanism only; it does NOT run just release itself — cutting the actual 0.12.0 release remains a manual step for Sahil.

State read at intake time (2026-09-23, not assumed):

- `.github/workflows/release.yml` `release` job, in order: checkout at the tag → setup-node 20 → setup-go → (dispatch-only `scripts/release.sh`) → resolve tag → setup-just → `npm ci` → `npm run build` → `just go-build-all` → `just go-package` → `just go-formula "<tag>"` + `cat dist/tu.rb` → `just release-notes` → `gh release create <tag> … dist/tu-go-*.tar.gz dist/tu-go-SHA256SUMS` → **"Update Homebrew tap"**: clone `sahil87/homebrew-tap` with `HOMEBREW_TAP_TOKEN`, `sed -i "s|tag: \"v.*\"|tag: \"v${version}\"|" /tmp/tap/Formula/tu.rb`, commit `tu ${version}`, push. The tap step is shared by all three entry paths (tag push, `workflow_dispatch`, release-labeled merge).
- The tap's `Formula/tu.rb` today is the **Node** formula: `url "https://github.com/sahil87/tu.git", using: :git, tag: "v0.12.0"`, `depends_on "node"`, `env :std`, `npm install --include=dev` + `npm run build` in `install`, `bin/tu` as a `write_env_script` over `libexec/tu.mjs`.
- `dist/tu.rb` (rendered by `scripts/go-formula.sh` from `.github/formula-template.rb`) is the **Go** formula: `version "<v>"`, four `on_macos`/`on_linux` × `on_arm`/`on_intel` blocks with `https://github.com/sahil87/tu/releases/download/v#{version}/tu-go-<os>-<arch>.tar.gz` URLs and sha256s, `libexec.install "tu", "vendor", "tu.default.conf"` + `bin.install_symlink libexec/"tu"`, no `depends_on`. Same shape as the tap's `fab-kit.rb`/`idea.rb`/`hop.rb`.
- fab-kit's tap step is the sibling pattern: `cp dist/fab-kit.rb /tmp/tap/Formula/fab-kit.rb` in place of the `sed`. R1's intake (line 143) already named this as X1's edit: "X1 replaces the `sed` with `cp dist/tu.rb /tmp/tap/Formula/tu.rb` (fab-kit's step, verbatim)".
- **Version reality differs from the plan text.** `v0.11.7` and `v0.12.0` were both cut on 2026-09-23 (10:57 and 10:59 IST, two `release:` commits, both pure `package.json`/lockfile bumps), both release runs green, both carrying `tu-go-*` assets — and the tap's `tu.rb` is now the Node formula at `tag: "v0.12.0"` (tap commit `df22882 tu 0.12.0`). So **0.12.0 has already shipped as a Node release**; the number is consumed. The first release cut after this change merges is the actual cutover and must be a minor bump per D9's rationale (a distribution change needs a minor bump under Output Stability): **0.13.0**. This intake is written for that; the plan doc's X1 row, Status line and D9 get a note (see § What Changes 4).

## Why

**The problem.** Every mechanism the Go binary needs to reach users exists and has been exercised on every release since R1 (2026-09-17): the four prebuilt tarballs are uploaded as release assets, `dist/tu.rb` is rendered from them and `cat` into the release log, R2 dogfooded the tarballs on a maintainer machine, and R3's full-matrix harness gate is green in CI (452/452 replay cases, 9/9 live-sync cases, zero unexpected diffs). The one thing that still decides what `tu update` installs is the tap step's `sed`, which bumps the tag inside the **Node** formula. Until that line changes, every release — including today's 0.12.0 — keeps installing the source-built Node bundle with `depends_on "node"`, and the Go port is invisible to users. G3 is the human stop before this; Sahil has queued X1.

**If we do not.** The port never ships. Phase 4 rows X2 (constitution v2), X3 (memory rehydrate) and X4 (shll's "tu exception") are all gated on X1, and Phase 5's `src/node/` removal is gated on two releases after it. The mixed-fleet window (TS 0.11.x/0.12.0 and Go co-writing the metrics repo, the real "both deployed" period D13 guards) cannot start until at least one Go release exists.

**Why this approach.** D13 chose shape A — a single formula flip, no side-by-side `tu-go`, no `TU_IMPL=node` escape hatch — because the harness gives byte-identical evidence before the flip, the user base is small, and rollback is one commit in the tap repo pointing `tu.rb` back at the Node formula (the Node tree stays buildable until Z1, D10). The mechanism is therefore the smallest possible edit: the tap step copies the already-generated `dist/tu.rb` over the tap's `Formula/tu.rb` instead of `sed`-bumping a tag. This is exactly fab-kit's step, and R1 built `dist/tu.rb` every release for a week precisely so X1 would reduce to this copy. Nothing about the Node lanes changes: `npm ci` is still required by the release job itself (`scripts/package-go.sh` reads the lockfile via `node -p` for the `CCUSAGE_VERSION` guard and the host smoke test; the justfile's `go_version` reads `package.json` via `node -p`), and `npm run build` stays as the fail-loud proof that the rollback artifact still builds. CI (`ci.yml`) is untouched — the harness still diffs `node dist/tu.mjs` against `bin/tu` on every PR.

**Why not a bump-type guard.** The idea of making the workflow refuse to push the Go formula unless the release is a minor bump was considered and rejected. The workflow cannot see the bump type on the tag-push path (only `workflow_dispatch` carries `inputs.bump`), a version-shape guard (`X.Y.0`) would have to consult tap state to know whether this is the *first* Go push, and — decisive — a guard that fails inside the release job fires **after** `scripts/release.sh` has already pushed the version commit and the `v*` tag, leaving a dangling tag to clean up. That is a worse mess than the thing it prevents (a Go formula under a patch number, which is a versioning-policy nit, not a broken install). The flip is unconditional from the first release after merge; the minor bump is the operator's `just release minor`, as the plan already states.

## What Changes

### 1. `release.yml` — the "Update Homebrew tap" step copies `dist/tu.rb`

The `sed` line is replaced by a copy. Everything else in the step (clone with `HOMEBREW_TAP_TOKEN`, bot identity, `git add Formula/tu.rb`, commit message `tu ${version}`, push) is unchanged, and the step stays **last** — after `gh release create` has uploaded the assets the formula's URLs point at, so no `brew install` can race a formula whose tarballs are not yet published.

```yaml
      - name: Update Homebrew tap
        env:
          TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
        run: |
          version="${{ steps.version.outputs.version }}"
          git clone "https://x-access-token:${TAP_TOKEN}@github.com/sahil87/homebrew-tap.git" /tmp/tap
          cp dist/tu.rb /tmp/tap/Formula/tu.rb
          cd /tmp/tap
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add Formula/tu.rb
          git commit -m "tu ${version}"
          git push
```

This is fab-kit's step verbatim with the file names substituted (`.github/workflows/release.yml` in `sahil87/fab-kit`, lines 115–127). No new guards, no `git diff --quiet` idempotency check — the siblings have none, and a re-run always produces a different formula because the tarball sha256s change per build.

Consequences of the copy, for the record:

- The tap's `tu.rb` becomes the Go formula on the first release after merge: `version "0.13.0"`, prebuilt tarball URLs, no `depends_on`, `libexec` + `bin` symlink install, `test do` asserting `tu --version`. `brew upgrade tu` (what `tu update` runs) sees `0.13.0 > 0.12.0` and upgrades; `node` is simply no longer a dependency of `tu` (Homebrew leaves it installed).
- The `/Cellar/tu/` gate in the Go `update` command passes (the symlinked binary lives under the keg's `libexec`), so `tu update` keeps working after the flip — the same layout R1 designed for.
- All three entry paths (tag push, `workflow_dispatch`, release-labeled merge) share the step, so the flip applies to each.

### 2. `release.yml` — comments and step name reflect the new state

- Header comment of the `release` job: the line "From plan row R1 the release also builds and uploads the tu-go-* Go assets (four tarballs + SHA256SUMS, D8); the Homebrew formula still ships the Node build until the cutover (row X1)." becomes a statement that from the cutover (plan row X1) the formula pushed to the tap is the generated Go formula (`dist/tu.rb`, prebuilt tarballs, no `depends_on "node"`), while the Node build still runs in the job as the fail-loud check that the rollback artifact (D10) builds.
- Step name `Generate Go formula (dist/tu.rb — not pushed until cutover, plan row X1)` becomes `Generate Go formula (dist/tu.rb — pushed to the tap below)`. The `cat dist/tu.rb` stays (the log shows exactly what was pushed).
- The comment above the fail-loud steps ("Install and build up front…") gains the reason `npm ci` survives the cutover: `package-go.sh` and the justfile read `package.json`/`package-lock.json` via `node -p`, and `npm run build` proves the Node rollback build.

### 3. Stale "not pushed until cutover" prose in touched build files

Comment-only edits, no behavior change:

- `scripts/go-formula.sh` header: "Generate the Go Homebrew formula into dist/tu.rb (NOT pushed to the tap until cutover, plan row X1)." → "Generate the Go Homebrew formula into dist/tu.rb — release.yml pushes it to sahil87/homebrew-tap (plan row X1)."
- `justfile`: the `go-formula` recipe comment ("written to dist/ and echoed in the release log, NOT pushed to the tap until cutover — plan row X1") and the Go section banner ("built and tested in CI, NOT shipped until cutover") are reworded to the shipped state: the Go binary is what the formula ships from the first release after X1; `src/node/` remains the harness oracle and rollback build until Z1.
- `scripts/release.sh`'s closing line "CI will … update the Homebrew tap." stays (still true).

Not touched: `scripts/package-go.sh`, `scripts/dogfood-install.sh` ("Go assets exist only for releases cut after plan row R1 landed" is still correct), `.github/formula-template.rb`, `ci.yml`, `README.md`, `docs/site/**`, `docs/specs/**`, `fab/project/constitution.md` and `fab/project/context.md` (their Go Transition prose is rewritten by X2, the next row).

### 4. Plan doc — record the version shift and the row status

`fab/plans/sahil/26-09-15-go-port.md` (the row's own status update, per the doc's header rule "update the status column as they land"):

- X1 row Status: "not started" → landed text naming this change and stating that 0.12.0 shipped as a Node release on 2026-09-23 before the flip was wired, so the cutover release is the **next minor bump, 0.13.0** (`git fetch origin && git checkout origin/main && just release minor`), still Sahil's manual step.
- The **Status (2026-09-23)** paragraph: "X1 = `just release minor` → 0.12.0" → 0.13.0, with the one-clause reason.
- D9 row: the decision text is not rewritten (D1–D13 are Sahil's to confirm); a trailing note in its Rationale cell records that 0.12.0 was consumed by a Node release on 2026-09-23 and the cutover number moved to 0.13.0. No pipe characters inside the cell.
- X2/X3/Z1 prose that says "after 0.12.0" / "two releases after cutover" is left as-is except Z1's "after 0.12.0", which becomes "after the cutover release (0.13.0)".

### 5. Memory (hydrate)

- `build/go-release-pipeline.md`: the "generated formula" requirement loses "not pushed"; the `release.yml` requirement gains the tap-copy step, its ordering after `gh release create`, and the shared-by-all-entry-paths note; a new short requirement "Rollback is one tap commit" (revert the tap's `tu.rb` to the Node formula at `tag: "v0.12.0"`; the next tu release re-pushes Go, so a rollback is paired with holding releases or reverting this change); the Overview drops "unshipped"; a Design Decision "Unconditional flip, no bump-type guard" in the four-field shape with the dangling-tag rationale above.
- `build/toolchain.md`: the `release.yml` bullet (tap step no longer `sed`s the Node formula) and the two "because the Homebrew formula builds from source on the user's machine" clauses (platform-package mapping, vendored-ccusage decision) get a present-truth rewrite: the formula now ships the prebuilt Go tarball with the target platform's ccusage vendored at package time; the from-source Node formula is the D10 rollback until Z1.

### Verification

There is no unit under test — the change is workflow YAML plus comments and docs. Apply verifies with:

- `python3 -c 'import yaml,sys; yaml.safe_load(open(".github/workflows/release.yml"))'` (or `actionlint` when installed) — the workflow still parses.
- `git diff --stat` shows `release.yml` changed only in the tap step, the header comment and the step name; no change under `src/`, `harness/`, `ci.yml`, `README.md`, `docs/site/`.
- `just go-dist` is **not** required (network + tag); `scripts/go-formula.sh` and `package-go.sh` are unchanged except the comment.
- The real proof is the next release run: the tap commit `tu 0.13.0` must contain a formula with `version "0.13.0"`, four sha256s equal to `dist/tu-go-SHA256SUMS`, and no `depends_on`. That run is Sahil's manual step and out of this change.

## Affected Memory

- `build/go-release-pipeline`: (modify) formula requirement no longer "unpushed"; tap-copy step and ordering; rollback requirement; "no bump-type guard" design decision
- `build/toolchain`: (modify) `release.yml` bullet (tap step copies `dist/tu.rb`), distribution and vendored-ccusage clauses that assume the from-source Node formula

## Impact

- **Files**: `.github/workflows/release.yml` (one `sed` → `cp` line, one step name, two comments), `scripts/go-formula.sh` (comment), `justfile` (two comments), `fab/plans/sahil/26-09-15-go-port.md` (X1 row, Status paragraph, D9 note, Z1 wording), two memory files + regenerated `docs/memory/build/log.md`.
- **External effect** (deferred to the first release after merge, cut by Sahil): `sahil87/homebrew-tap` `Formula/tu.rb` becomes the Go formula; every user's next `tu update` installs the prebuilt Go binary with vendored ccusage. This is the plan's "first moment the normal update path delivers it."
- **Unchanged surfaces**: everything under the plan's Goal (CLI grammar, `--help`, output formats, exit codes, conf files, metrics-repo layout, toolkit contracts); `ci.yml` and the harness; `README.md` install prose and `docs/site/install.md` (neither mentions node); the `tu-go-*` asset contract; `scripts/release.sh`; `.github/formula-template.rb`.
- **Rollback**: one commit in `sahil87/homebrew-tap` restoring the Node `tu.rb` (`tag: "v0.12.0"`), then `tu update` on affected machines. The next tu release would re-push the Go formula, so a rollback that must hold requires reverting this change in tu as well.
- **Risk**: the first real execution of the copy is the cutover release itself. Mitigation: the step is the sibling's proven step; the formula it copies has been rendered and logged on every release since R1 (v0.11.6, v0.11.7, v0.12.0); the step runs after the Release exists, so a failure is "tap not updated" (re-runnable), never "formula points at missing assets".
- **Follow-ons unblocked**: X2 (constitution v2), X3 (memory rehydrate), X4 (shll tu-exception) run unattended after this merges; Z1 counts two releases from 0.13.0.

## Open Questions

None.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Tap step becomes `cp dist/tu.rb /tmp/tap/Formula/tu.rb` in place of the `sed` tag bump; clone, bot identity, `tu ${version}` commit and push unchanged | Named verbatim by R1's intake as X1's edit; fab-kit's step is the sibling pattern; the generated formula already carries version and sha256s | S:90 R:90 A:95 D:95 |
| 2 | Certain | Tap step stays last, after `gh release create` | The formula URLs point at release assets; publishing the formula first would let a `brew install` 404. Current ordering already guarantees this | S:80 R:85 A:95 D:95 |
| 3 | Confident | Cutover release is the next minor bump, **0.13.0**, not 0.12.0; plan doc X1 row, Status line and D9 note record the shift | 0.12.0 shipped as a Node release today (tap commit `df22882`, release run green) before the flip existed; D9's rationale (distribution change needs a minor bump) still holds, only the number moves; re-cutting 0.12.0 would mean deleting a published tag/release and a tap commit users may already have installed | S:35 R:90 A:65 D:60 |
| 4 | Confident | The flip is unconditional from the first release after merge — no bump-type or version-shape guard in the workflow | Tag-push path has no bump type; a `X.Y.0` guard needs tap state to know it is the first Go push; any in-job guard fires after `release.sh` pushed the tag, leaving a dangling tag — worse than a patch-numbered Go formula | S:40 R:85 A:60 D:55 |
| 5 | Certain | `npm ci` and `npm run build` stay in the `release` job | `package-go.sh` (lockfile guard, host smoke) and the justfile's `go_version` read `package.json`/lockfile via `node -p`; the build proves the D10 rollback artifact still builds; the raw input says Node steps stay in CI | S:75 R:90 A:85 D:80 |
| 6 | Confident | No `brew install` smoke of the pushed formula is added to the workflow; cutover criterion 5 stays a by-hand check | Raw input scopes this to "wire the mechanism only"; no sibling workflow smoke-installs its formula; adds a slow external dependency to every release | S:45 R:85 A:60 D:60 |
| 7 | Certain | `constitution.md` and `context.md` Go Transition prose are left to X2; README and `docs/site/install.md` untouched | Plan row X2 owns the constitution rewrite; the raw input freezes README (Policy B); `install.md` never mentions node | S:70 R:90 A:85 D:85 |
| 8 | Certain | Comment-only sweep of "not pushed until cutover" in `release.yml`, `scripts/go-formula.sh`, `justfile`; no other files | Present-truth hygiene for the files the change already touches; anything wider is X3's rehydrate | S:60 R:95 A:80 D:80 |
| 9 | Certain | Change type `ci` (workflow + release plumbing), pinned explicitly | The change is entirely release-workflow wiring; inference from the raw text lands on `feat` | S:70 R:95 A:85 D:85 |

9 assumptions (6 certain, 3 confident, 0 tentative, 0 unresolved).
