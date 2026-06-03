# Intake: Remove shll.ai help-dump push wiring (shll.ai now pulls)

**Change**: 260603-dmhw-remove-shll-ai-push-wiring
**Created**: 2026-06-03
**Status**: Draft

## Origin

> There's an update in the way we integrate with shll.ai. To understand it read
> https://github.com/sahil87/shll.ai/blob/main/docs/specs/help-dump-contract.md#teardown-directive-paste-to-a-tool-repo-agent
> Implement the change.

One-shot invocation. The referenced section is the **Teardown directive (paste to a tool-repo
agent)** in shll.ai's `help-dump-contract.md`, added by shll.ai change `oa63` on 2026-06-03. shll.ai
has **inverted** how it collects command-reference JSON: it used to *receive* each tool's data via a
**push** (the tool's CI generated `help/<tool>.json` and opened an auto-merged PR into `sahil87/shll.ai`
using a cross-repo `SHLLAI_TOKEN`). It now **pulls** — a scheduled job in shll.ai
(`scheduled-help-refresh.yml`) `brew install`s each tool, runs `<tool> help-dump`, stamps `captured_at`,
validates, and direct-commits the result to its own `main`. tu's job is to remove the now-dead push
wiring in a single PR, **without touching the `help-dump` command** (the contract surface shll.ai now
depends on).

**Verified before scoping** (not assumed):
1. **Puller is live + proven** (the directive's hard precondition). `sahil87/shll.ai` contains
   `.github/workflows/scheduled-help-refresh.yml` whose header states it "Replaces the retired push
   model (help-automerge.yml)", uses the default `GITHUB_TOKEN` with `contents:write` (no
   `SHLLAI_TOKEN`), and direct-commits. Its `help/` directory already holds 6 of 7 tools
   (`fab-kit, hop, idea, run-kit, shll, wt`). `help/tu.json` is absent — and the puller explicitly
   documents "A MISSING help/tu.json is the expected interim state (tu's help-dump is in progress),
   not a failure." Tearing out tu's push is therefore the intended next step and leaves no stale-help gap.
2. **`SHLLAI_TOKEN` is used in exactly one place** — `grep -rn SHLLAI_TOKEN` returns only the single
   `release.yml` step (lines 214/217/218) plus documentation/backlog/plan references (not live wiring).
3. **The transport and the producer-step both live inside `release.yml`** — there is no separate
   producer workflow file, no separate PR-opening or auto-merge workflow. tu's wiring is two adjacent
   steps in the `release` job: `Generate help/tu.json` (`npm run help-dump`) and `PR help/tu.json into
   shll.ai`.
4. **The `help-dump` command stays** — `scripts/help-dump.mjs` (the producer *script*, run via
   `npm run help-dump`) and its unit test `src/node/core/__tests__/help-dump.test.ts` are out of scope.

## Why

1. **Problem**: tu still *pushes* `help/tu.json` into `sahil87/shll.ai` on every release (clone + branch
   + commit + `gh pr create` + `gh pr merge --auto --squash`, authed with `SHLLAI_TOKEN`). shll.ai has
   inverted the contract and now pulls this data on its own daily schedule. The push side is dead
   wiring: redundant work, a standing cross-repo write credential (`SHLLAI_TOKEN`) that no longer needs
   to exist, and a second writer racing the puller for the same `help/tu.json` on shll.ai's `main`.
2. **Consequence if not removed**: two systems write the same file on shll.ai. tu's push PR and the
   shll.ai cron can interleave (the very multi-repo race the original PR-with-auto-merge design tried to
   tame — now moot, because the single trusted cron is the only writer the new model wants). The
   `SHLLAI_TOKEN` cross-repo write secret lingers in tu's Actions secrets with no live consumer — an
   unnecessary credential and attack surface. Every tu release does avoidable work and can fail on
   shll.ai-side branch/merge conditions tu no longer controls.
3. **Why this approach**: the teardown directive is explicit and gated — remove only the *transport*
   (the CI that pushed the output), never the command that *produces* it. The producer script and its
   test stay so the contract surface shll.ai pulls from is preserved exactly. This is the minimal,
   single-PR change the directive prescribes; the precondition (puller live + proven) is verified, so
   it is safe now.

## What Changes

All changes are in **`.github/workflows/release.yml`**, inside the `release` job. The `help-dump`
command, its npm script, and its test are explicitly **not** touched.

### 1. Delete the push step — `PR help/tu.json into shll.ai`

Remove the entire terminal step (`release.yml` lines 212–249). This single step *is* the producer-CI
transport + PR-opening + auto-merge + `SHLLAI_TOKEN` usage that the directive's four deletable items
enumerate (here they are not four separate files — they collapse into this one step):

- clone of `sahil87/shll.ai` via `https://x-access-token:${SHLLAI_TOKEN}@…`
- branch `tu-help-dump-v${version}`, `git add help/tu.json`, commit, `git push --force-with-lease`
- `gh pr create … --base main --head "$branch"`
- `gh pr merge --auto --squash "$branch"`
- the `SHLLAI_TOKEN` / `GH_TOKEN: ${{ secrets.SHLLAI_TOKEN }}` env block

### 2. Delete the producer step — `Generate help/tu.json`

Remove the `Generate help/tu.json` step (`release.yml` lines 180–181, `run: npm run help-dump`). With
the push gone, this step has **no consumer** in `release.yml` (the artifact it writes was only ever
copied by the push step), and `help/` is gitignored so nothing else reads it. Decision confirmed with
the user: delete it (do not retain it as a smoke test). The producer *script* (`scripts/help-dump.mjs`)
and the npm `help-dump` script entry stay — only the workflow *step* that invoked it is removed.

### 3. Update the fail-loud rationale comment

The comment block at `release.yml` lines 167–173 ("Fail-loud steps FIRST … generate+validate
help/tu.json up front … avoiding a partially-applied release") exists to justify ordering the help-dump
generation before the GitHub Release / Homebrew side effects. With both help-dump steps gone, that
rationale is stale. Trim the comment so it accurately describes the remaining fail-loud steps (install +
build) without dangling references to help-dump.

### 4. Leave the rest of `release.yml` intact

Per directive item 5: the help-push was steps *inside* a larger workflow, not a standalone file — so
remove only those steps/comments and leave everything else untouched: `tag-on-release-merge`, tag
resolution, `Install dependencies`, `Build bundle`, `Generate release notes`, `Create GitHub Release`,
and `Update Homebrew tap` are all unaffected. `release.yml` is **not** deleted.

### 5. Flag the `SHLLAI_TOKEN` repo secret for manual removal

Per directive item 4: after removing `SHLLAI_TOKEN` usage from the workflow, the repo secret itself
should be removed **only after confirming it is not used anywhere else**. The full-repo grep confirms
the only live wiring reference is the step being deleted (remaining hits are docs/backlog/plan). The
secret removal is a GitHub repo-settings action this change cannot perform from code — it is **flagged
for the user** as a manual follow-up, not performed here.

### 6. The `help-dump` command — preserved invariant (no change)

`scripts/help-dump.mjs` continues to emit the frozen contract per shll.ai's `help-dump-contract.md`:
`Hidden`/self-filtering behavior, `{tool, version, schema_version: 1, root}` envelope to stdout,
`schema_version: 1`, version from the built binary. The directive notes a *future* shll.ai change will
enrich the schema with new optional fields and migrate `captured_at`/stdout ownership — **explicitly not
part of this work**; tu keeps emitting `schema_version: 1` exactly as today. The existing unit test
(`help-dump.test.ts`, run in `ci.yml` via `npm test`) already protects this surface, so **no new test is
added** — the directive's "add a minimal test if none exists" branch does not apply.

## Affected Memory

- `build/toolchain`: (modify) The "Help-dump → shll.ai (build-time CLI help artifact)" section
  documents the **push** model being removed. Update it to reflect the pull inversion: tu no longer
  pushes; `release.yml` no longer runs the producer or PRs into shll.ai; `SHLLAI_TOKEN` usage removed;
  the `help-dump` producer script + contract are retained as the surface shll.ai now pulls. Revise the
  `release.yml` bullet (item describing the PR + auto-merge + `SHLLAI_TOKEN`), the design-decision
  bullets about "PR-with-auto-merge into shll.ai", and add a changelog entry. (Hydrate stage.)

## Impact

- **`.github/workflows/release.yml`** — remove two steps + trim one comment block (the only code change).
- **`scripts/help-dump.mjs`** — unchanged (preserved contract surface).
- **`package.json`** `help-dump` script — unchanged (still invokable locally / by future tooling).
- **`src/node/core/__tests__/help-dump.test.ts`** — unchanged (continues to guard the contract).
- **`.github/workflows/ci.yml`** — unchanged (never ran help-dump; runs build + tests).
- **GitHub repo settings** — `SHLLAI_TOKEN` secret flagged for manual deletion (out-of-band).
- **`docs/memory/build/toolchain.md`** — updated at hydrate.
- **`sahil87/shll.ai`** — no longer receives tu PRs; its cron puller already covers `help/tu.json`.

## Open Questions

(None — the puller-live precondition is verified, the directive is explicit, and the one design choice
[delete vs. keep the `Generate help/tu.json` step] was confirmed with the user.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Remove only the *transport* (the two help-dump steps in `release.yml`); never touch the `help-dump` command, its npm script, or its test | Directive's central invariant, stated verbatim ("removes only the transport … never the command that produces it"); the test already runs in `ci.yml` so the contract stays protected | S:98 R:80 A:95 D:95 |
| 2 | Certain | Delete the entire `PR help/tu.json into shll.ai` step (clone + branch + commit + push + `gh pr create` + `gh pr merge --auto` + `SHLLAI_TOKEN` env) | This single step IS the directive's four deletable items (producer-transport, PR-opening, auto-merge, token) collapsed into one; grep confirms no other live wiring | S:97 R:80 A:95 D:92 |
| 3 | Certain | Teardown is safe now — the shll.ai puller is live and proven | Verified directly: `scheduled-help-refresh.yml` exists in shll.ai, replaces the push model, direct-commits with `GITHUB_TOKEN`, and already holds 6/7 tools; missing `help/tu.json` is the documented expected interim state | S:95 R:60 A:90 D:90 |
| 4 | Confident | Also delete the `Generate help/tu.json` step (`npm run help-dump`) — it has no remaining consumer once the push is gone | Confirmed with the user (delete over keep-as-smoke-test). The artifact is gitignored and was only read by the push step; the producer logic stays protected by the unit test in `ci.yml`. Strictly, directive item 1 ("delete the producer CI step") covers this | S:90 R:75 A:85 D:80 |
| 5 | Confident | This repo's `<tool>` is `tu`, not `fab` | `package.json` name is `tu`, bin `dist/tu.mjs`, artifact `help/tu.json`; `fab` is the fab-kit repo's binary. The directive's `<tool>` placeholder resolves to the repo's actual binary | S:95 R:85 A:90 D:90 |
| 6 | Confident | Trim the lines 167–173 "Fail-loud steps FIRST" comment to drop stale help-dump references | The comment's stated purpose (order help-dump generation before external side effects) vanishes with the steps; leaving it would misdescribe the workflow. Low blast radius (a comment) | S:85 R:90 A:85 D:75 |
| 7 | Confident | `release.yml` is NOT deleted — only steps/comments are removed | Directive item 5: help-push was steps inside a larger workflow, so remove just those and leave the rest (tagging, release, Homebrew tap) intact | S:95 R:80 A:90 D:90 |
| 8 | Confident | `SHLLAI_TOKEN` repo secret removal is flagged as a manual follow-up, not done in this change | Directive says remove the secret only after confirming no other use (grep confirms none live); deleting a GitHub Actions secret is a repo-settings action, not a code change this PR can make | S:90 R:70 A:80 D:80 |
| 9 | Confident | No new `help-dump` test is added | Directive adds a minimal test only "if you have none"; one already exists (`help-dump.test.ts`) and runs in `ci.yml` — the contract surface stays protected without duplication | S:92 R:80 A:90 D:85 |
| 10 | Certain | Keep emitting `schema_version: 1`; the future optional-field schema enrichment is out of scope | Directive: "A separate, future shll.ai change will enrich the help-dump schema … that is not part of this work — keep emitting schema_version: 1 as today" | S:98 R:85 A:95 D:95 |

10 assumptions (5 certain, 5 confident, 0 tentative, 0 unresolved).
