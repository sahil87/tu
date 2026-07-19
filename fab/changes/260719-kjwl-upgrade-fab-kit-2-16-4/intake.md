# Intake: Upgrade fab-kit to 2.16.4

**Change**: 260719-kjwl-upgrade-fab-kit-2-16-4
**Created**: 2026-07-19

## Origin

> fab upgrade-repo, then drive the resulting change through the full pipeline with /fab-fff. If fab upgrade-repo produced no diff, stop — do not run /fab-fff and do not run /git-pr.

One-shot invocation. The user ran the pipeline conditionally on the upgrade producing a diff — it did (3 files), so the full pipeline proceeds. Key decisions from the conversation:

- `fab upgrade-repo` was executed **before** this change was created — the mechanical upgrade is already applied to the working tree. The pipeline's job is to verify, document, and ship it, not to re-run it.
- The user explicitly requested `/fab-fff` (full pipeline through ship and review-pr), not `/fab-ff`.

## Why

1. **Problem**: The repo's fab-kit deployment was at 2.16.0 while the installed fab-kit is 2.16.4. Stale kit deployments drift from the current skill files, templates, and migration state — the upgrade run also repaired 6 Claude Code skill files that had drifted from their kit-canonical content.
2. **Consequence of not upgrading**: The repo's deployed skills/templates progressively diverge from the kit the CLI expects, and pending kit migrations (2.15.8 → 2.16.4) never apply, risking incompatibility with newer `fab` CLI behavior.
3. **Approach**: `fab upgrade-repo` is the kit's own single supported upgrade path — it resolves the target kit version, runs migrations, syncs deployed agent files, and regenerates the config reference fence. No alternative was considered because none exists; this mirrors the previous upgrade shipped as commit `2d5cfdc` ("chore: Upgrade fab-kit to 2.16.0 (#53)").

## What Changes

The upgrade has **already been executed** by `fab upgrade-repo`. The resulting working-tree diff is exactly 3 files, 3 lines:

### Version stamps

- `fab/.fab-version`: `2.16.0` → `2.16.4` (the deployed kit version)
- `fab/.kit-migration-version`: `2.15.8` → `2.16.4` (kit migrations through 2.16.4 have been applied)

### Config reference fence

- `fab/project/config.yaml`: the auto-regenerated reference fence header updates from `# >>> fab reference (kit 2.16.0) >>>` to `# >>> fab reference (kit 2.16.4) >>>`. No user-owned config values above the fence changed.

### Not in the git diff (informational)

The upgrade run also reported `Claude Code: 34/34 (created 0, repaired 6, already valid 28)` — 6 deployed skill files under `.claude/` were repaired to kit-canonical content. These paths are not tracked by git (not in the diff), so they ship nothing but explain why the upgrade matters beyond the 3 stamp lines.

### Remaining pipeline work

- Verify the diff is exactly the 3 files above (no strays).
- Run the test suite to confirm repo health under the upgraded kit.
- Ship as a `chore:` PR, mirroring commit `2d5cfdc`.

## Affected Memory

None — `fab/` is pipeline infrastructure, not project behavior. No `docs/memory/` domain (build, cli, configuration, display, sync, watch-mode) describes fab-kit versioning, and `true_impact_exclude` already excludes `fab/`. Implementation-only change; no spec-level behavior of `tu` is touched.

## Impact

- **Files**: `fab/.fab-version`, `fab/.kit-migration-version`, `fab/project/config.yaml` (fence comment only) — 3 lines total.
- **Source code**: none (`src/` untouched). No API, dependency, or output changes.
- **Tests**: none affected; the suite runs as a health check only.
- **Systems**: fab pipeline tooling only. Zero user-facing impact on the `tu` CLI.

## Open Questions

None.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Change type is `chore` | Tooling-version bump with no source changes; precedent commit `2d5cfdc` used `chore:` for the identical 2.16.0 upgrade | S:90 R:90 A:95 D:95 |
| 2 | Certain | No memory hydration needed | `fab/` is pipeline infra excluded by `true_impact_exclude`; no memory domain covers kit versioning | S:80 R:90 A:90 D:85 |
| 3 | Confident | Apply verifies the already-applied diff instead of re-running `fab upgrade-repo` | The upgrade (incl. migrations) already ran in this session; re-running would be a no-op at best and risks double-applying migrations | S:75 R:85 A:80 D:75 |
| 4 | Certain | PR ships as `chore: Upgrade fab-kit to 2.16.4` | Mirrors the merged precedent PR #53 title for 2.16.0 | S:70 R:95 A:90 D:85 |

4 assumptions (3 certain, 1 confident, 0 tentative, 0 unresolved).
