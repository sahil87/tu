# Spec: Relicense MIT & Migrate to sahil87

**Change**: 260401-lomt-relicense-mit-migrate-sahil87
**Created**: 2026-04-01
**Affected memory**: `docs/memory/build/toolchain.md`

## Non-Goals

- Modifying weaver config files (`tu.default.weaver.conf`, `tu.default.conf`) — these reference a functional private metrics repo under wvrdz, not branding
- Updating existing change artifacts in `fab/changes/` — historical records stay as-is
- Adding `repository` or `homepage` fields to `package.json` — fields don't exist today and aren't needed

## Licensing

### Requirement: MIT License File

The `LICENSE` file SHALL contain the full MIT license text with copyright holder `Sahil Ahuja` and year `2026`. The previous PolyForm Internal Use 1.0.0 content SHALL be replaced entirely.

#### Scenario: License file contents
- **GIVEN** the `LICENSE` file exists at the repo root
- **WHEN** a user reads the file
- **THEN** the file contains the standard MIT license text
- **AND** the copyright line reads `Copyright (c) 2026 Sahil Ahuja`

### Requirement: Package License Field

The `license` field in `package.json` SHALL be `"MIT"`. No other fields in `package.json` SHALL be added or removed by this change.

#### Scenario: package.json license value
- **GIVEN** `package.json` exists at the repo root
- **WHEN** the `license` field is read
- **THEN** its value is `"MIT"`

## Org Migration

### Requirement: CLI Update Message

The CLI update instruction in `src/node/core/cli.ts` SHALL reference `sahil87/tap/tu` instead of `wvrdz/tap/tu`.

#### Scenario: Update message shows sahil87 tap
- **GIVEN** the CLI displays an update instruction
- **WHEN** the user reads the update message
- **THEN** it contains `brew install sahil87/tap/tu`
- **AND** no reference to `wvrdz/tap` appears in the update message

### Requirement: README Install Instructions

The `README.md` install section SHALL reference the public `sahil87/tap` without requiring SSH access. The `brew tap` command SHALL use `sahil87/tap` (no SSH URL). The note about SSH access to wvrdz SHALL be removed.

#### Scenario: Public install instructions
- **GIVEN** `README.md` exists at the repo root
- **WHEN** a user follows the install instructions
- **THEN** the tap command is `brew tap sahil87/tap`
- **AND** no SSH URL or wvrdz reference appears in the install section

## Project Documentation

### Requirement: Context File Updates

`fab/project/context.md` SHALL reflect the current distribution and license:
- Distribution line SHALL reference `sahil87/tap`
- License line SHALL read `MIT`

#### Scenario: Context file reflects new org
- **GIVEN** `fab/project/context.md` describes the project stack
- **WHEN** the distribution and license lines are read
- **THEN** distribution references `sahil87/tap` (not `wvrdz/tap`)
- **AND** license reads `MIT` (not `PolyForm Internal Use 1.0.0`)

### Requirement: Memory File Updates

`docs/memory/build/toolchain.md` SHALL reflect the current org, license, and distribution:
- All references to `wvrdz/tap` SHALL become `sahil87/tap`
- All references to `wvrdz/homebrew-tap` SHALL become `sahil87/homebrew-tap`
- References to `wvrdz` GitHub org (in distribution context) SHALL become `sahil87`
- License references SHALL change from `PolyForm-Internal-Use-1.0.0` to `MIT`
- The "SSH-gated access" note SHALL be removed (tap is now public)

#### Scenario: Toolchain memory reflects migration
- **GIVEN** `docs/memory/build/toolchain.md` describes build and distribution
- **WHEN** the file is read
- **THEN** all distribution references use `sahil87` org
- **AND** the license is listed as `MIT`
- **AND** no SSH access requirement is mentioned for the tap

## Preserved References

### Requirement: Weaver Config Unchanged

`tu.default.weaver.conf` and `tu.default.conf` SHALL NOT be modified. The `metrics_repo` value pointing to `wvrdz/tu-metrics.git` is a functional reference to a private repo, not branding.

#### Scenario: Weaver config preserved
- **GIVEN** `tu.default.weaver.conf` contains `metrics_repo = git@github.com:wvrdz/tu-metrics.git`
- **WHEN** this change is applied
- **THEN** the file is unmodified

### Requirement: Test File Unchanged

`src/node/core/__tests__/config.test.ts` SHALL NOT be modified. It asserts the weaver config's `metricsRepo` value which correctly points to the wvrdz repo.

#### Scenario: Config test preserved
- **GIVEN** the config test asserts the weaver metricsRepo value
- **WHEN** this change is applied
- **THEN** the test file is unmodified
- **AND** the test continues to pass

### Requirement: Historical Artifacts Unchanged

All files under `fab/changes/` (excluding the current change folder) SHALL NOT be modified. These are historical records.

#### Scenario: Archived changes preserved
- **GIVEN** existing change folders contain wvrdz references
- **WHEN** this change is applied
- **THEN** those files remain unmodified

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | License changes to MIT | Confirmed from intake #1 — user explicitly chose MIT | S:95 R:90 A:95 D:95 |
| 2 | Certain | Org changes from wvrdz to sahil87 | Confirmed from intake #2 — repo already exists at sahil87/tu | S:95 R:85 A:95 D:95 |
| 3 | Certain | Weaver config stays as-is | Confirmed from intake #3 — functional reference, not branding | S:85 R:90 A:90 D:90 |
| 4 | Certain | Historical artifacts untouched | Confirmed from intake #4 — falsifying records is wrong | S:90 R:95 A:90 D:95 |
| 5 | Confident | No new fields added to package.json | Confirmed from intake #5 — keep minimal, not needed | S:70 R:95 A:80 D:80 |
| 6 | Certain | Copyright holder is "Sahil Ahuja" | Confirmed from intake #6 — consistent with existing MIT in homebrew-tap | S:90 R:90 A:90 D:95 |
| 7 | Certain | SSH access note removed from README | Public repo needs no SSH — derived from migration context | S:90 R:90 A:95 D:95 |

7 assumptions (6 certain, 1 confident, 0 tentative, 0 unresolved).
