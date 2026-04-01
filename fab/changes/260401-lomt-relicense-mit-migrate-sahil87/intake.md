# Intake: Relicense MIT & Migrate to sahil87

**Change**: 260401-lomt-relicense-mit-migrate-sahil87
**Created**: 2026-04-01
**Status**: Draft

## Origin

> Migrate tu from wvrdz to sahil87: change license from PolyForm-Internal-Use-1.0.0 to MIT, update all hardcoded wvrdz/tu references to sahil87/tu. This is the "prep the tu codebase" step — the GitHub repo (sahil87/tu) already exists and the Homebrew formula has already been moved to sahil87/homebrew-tap.

Conversational mode — preceded by a `/fab-discuss` session where the full migration plan was laid out and steps 2–4 (repo creation, formula migration, wvrdz wind-down) were already executed.

## Why

The project is being open-sourced under the sahil87 GitHub org with an MIT license. The wvrdz org was the original home (private, PolyForm-licensed). The repo has already been created at sahil87/tu (public) and the Homebrew formula moved to sahil87/homebrew-tap. The codebase itself still contains the old license and org references throughout — this change brings the source in line with the new reality.

Without this change, the LICENSE file contradicts the repo's public status, `brew install` instructions point to a tap that no longer carries the formula, and docs/memory reference an org the public user can't access.

## What Changes

### License file

Replace the full PolyForm Internal Use 1.0.0 text in `LICENSE` with the MIT license. Copyright holder: `Sahil Ahuja`.

### package.json

Change the `license` field from `"PolyForm-Internal-Use-1.0.0"` to `"MIT"`. No `repository` or `homepage` fields exist currently — no additions needed (keep it minimal).

### CLI update message

In `src/node/core/cli.ts:248`, the update instruction reads:

```
brew install wvrdz/tap/tu
```

Change to:

```
brew install sahil87/tap/tu
```

### README.md

Rewrite the install section. Current text references SSH access to the wvrdz org and `brew tap wvrdz/tap git@github.com:wvrdz/homebrew-tap.git`. Replace with:

```
brew tap sahil87/tap
brew install tu
```

No SSH URL needed — the tap is public.

### Default config files

- `tu.default.weaver.conf:14` — `metrics_repo = git@github.com:wvrdz/tu-metrics.git`. This is a functional config value pointing to a real private metrics repo. **Leave as-is** — this is the weaver (internal dev) config, not the user-facing default. The metrics repo stays under wvrdz.
- `tu.default.conf:14` — commented-out `# metrics_repo = ...`. Same — leave as-is.

### Test file

- `src/node/core/__tests__/config.test.ts:246` — asserts the weaver config's `metricsRepo` value. Stays as-is since the weaver config is unchanged.

### Project docs (fab/project/)

- `fab/project/constitution.md` — no explicit license string to change (constitution references "Single-Bundle Distribution" but not the license identifier)
- `fab/project/context.md:10–11` — update distribution line from `wvrdz/tap` to `sahil87/tap` and license line from `PolyForm Internal Use 1.0.0` to `MIT`

### Memory files (docs/memory/)

- `docs/memory/build/toolchain.md` — references `wvrdz/tap`, `wvrdz/homebrew-tap`, `wvrdz` GitHub org, and `PolyForm-Internal-Use-1.0.0`. Update all to reflect sahil87 org and MIT license.
- `docs/memory/configuration/config-system.md` — mentions "wvrdz metrics repo" in passing. Leave as-is — it's describing the weaver config which genuinely points to wvrdz.

### Existing change artifacts (fab/changes/)

Leave all references in existing change folders (archived or in-progress) untouched. These are historical records of what was true at the time.

## Affected Memory

- `build/toolchain`: (modify) Update org references from wvrdz to sahil87, license from PolyForm to MIT, tap from wvrdz/tap to sahil87/tap

## Impact

- **Source files touched**: `LICENSE`, `package.json`, `README.md`, `src/node/core/cli.ts`
- **Project docs touched**: `fab/project/context.md`
- **Memory files touched**: `docs/memory/build/toolchain.md`
- **No behavioral changes** — no logic, data model, or output format changes
- **package-lock.json** will auto-update when `npm install` runs (picks up license field change)

## Open Questions

None — scope is well-defined from the preceding discussion.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | License changes to MIT | Discussed — user explicitly chose MIT for sahil87 projects | S:95 R:90 A:95 D:95 |
| 2 | Certain | Org changes from wvrdz to sahil87 | Discussed — user migrating projects to sahil87, repo already created | S:95 R:85 A:95 D:95 |
| 3 | Certain | Weaver config (tu.default.weaver.conf) stays as-is | metrics_repo points to a real private wvrdz repo — functional, not branding | S:85 R:90 A:90 D:90 |
| 4 | Certain | Existing change artifacts left untouched | Historical records — modifying them would falsify the record of what was decided at the time | S:90 R:95 A:90 D:95 |
| 5 | Confident | No repository/homepage fields added to package.json | User said "keep it minimal"; fields don't exist today and aren't needed for npm or Homebrew | S:70 R:95 A:80 D:80 |
| 6 | Certain | Copyright holder is "Sahil Ahuja" | MIT license in sahil87/homebrew-tap already uses this — consistent | S:90 R:90 A:90 D:95 |

6 assumptions (5 certain, 1 confident, 0 tentative, 0 unresolved).
