# Intake: CI Gate for PR Merges

**Change**: 260604-5wf9-ci-gate-pr-merges
**Created**: 2026-06-04
**Status**: Draft

## Origin

This change began as a `/fab-new` invocation with the raw user input:

> Add ci/cd steps for every merge - that make sure the test cases pass any PR merges. We can add a ci-gate step if needed

The session was **conversational**: before generating this intake, the agent performed a gap
analysis against the existing CI/CD setup and surfaced two design choices to the user.

**Decisions reached during the conversation:**

1. **CI trigger** — Extend the existing `ci.yml` to run on **both** `pull_request: branches: [main]`
   (to gate merges) **and** the current `push: branches: [main]` (post-merge safety net + status on
   the release-merge commit). The user chose this over a `pull_request`-only trigger to keep the
   existing post-merge verification intact.

2. **Merge gate** — Add an explicit aggregating **`ci-gate` job** (a single, stable status-check
   name to require) AND **auto-apply a GitHub ruleset** during the change that gates PRs on
   `ci-gate`. The user's note was explicit: *"Also add the required ruleset that gates PRs on
   ci-gate."* — so the change both provides the check (workflow) and enforces it (ruleset), rather
   than only documenting a manual branch-protection step.

## Why

**The problem.** The repository's only test-running CI (`.github/workflows/ci.yml`) triggers solely
on `push: branches: [main]` — i.e., *after* code has already landed on `main`. It verifies the build
and runs `npm test`, but it does so too late to prevent a broken change from being merged. There is
**no branch protection** on `main` (confirmed: `GET .../branches/main/protection` returns 404), so
even a red post-merge CI run blocks nothing — `main` can break and stay broken until someone notices.

**The consequence if unfixed.** Contributors (and automated agents opening PRs, e.g. the help-dump
flow) can merge changes that fail the test suite or break the esbuild bundle. `main` is the release
anchor — `release.yml` tags and ships from it — so a broken `main` directly risks shipping a broken
Homebrew bundle. The cost of a regression scales with how long it sits undetected on `main`.

**Why this approach.** Running CI `on: pull_request` is the standard GitHub mechanism for
pre-merge verification; it gives every PR a status check reflecting build + test health *before*
merge. A workflow alone cannot block a merge, though — enforcement requires a repo-level rule. So the
change pairs the PR-triggered workflow with a **required status check** (via a GitHub ruleset) on
`main`. An explicit `ci-gate` aggregating job gives that rule one stable check name to require, which
stays valid even if individual jobs are renamed or split later. The existing `push: [main]` trigger
is retained so the merge commit itself is still verified and so the release-merge path keeps its
status signal.

## What Changes

### 1. Extend `ci.yml` triggers to run on pull requests

Modify `.github/workflows/ci.yml` so it runs on both pull requests targeting `main` and pushes to
`main`:

```yaml
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
```

The existing `build-and-test` job (checkout → setup-node 20 → `npm ci` → `npm run build` →
`npm test`) is unchanged in its steps; it now simply runs in the PR context as well. The action SHAs
remain pinned (kept in sync with `release.yml` as the existing header comment notes).

The leading comment block in `ci.yml` is updated to reflect that the workflow now gates PRs in
addition to verifying pushes to `main`.

### 2. Add an aggregating `ci-gate` job

Add a lightweight job that depends on `build-and-test` and succeeds only when it succeeded. This is
the single, stable status-check name the branch ruleset requires:

```yaml
  ci-gate:
    needs: [build-and-test]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - name: Verify required jobs passed
        run: |
          if [ "${{ needs.build-and-test.result }}" != "success" ]; then
            echo "build-and-test did not succeed (result: ${{ needs.build-and-test.result }})"
            exit 1
          fi
          echo "All required CI jobs passed."
```

`if: always()` ensures `ci-gate` runs (and can report a failure) even when `build-and-test` fails or
is cancelled, so the required check resolves to a definitive pass/fail rather than being skipped.

### 3. Apply a GitHub ruleset requiring `ci-gate` on `main`

During apply, configure a branch ruleset on `main` (repo `sahil87/tu`) that requires the `ci-gate`
status check to pass before a PR can be merged. Implementation via `gh api` against the **rulesets**
endpoint (confirmed with the user — the modern, GitHub-recommended mechanism; not the classic
branch-protection endpoint):

```sh
gh api -X POST repos/sahil87/tu/rulesets \
  --input - <<'JSON'
{
  "name": "Require CI gate on main",
  "target": "branch",
  "enforcement": "active",
  "conditions": { "ref_name": { "include": ["refs/heads/main"], "exclude": [] } },
  "rules": [
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": false,
        "required_status_checks": [ { "context": "ci-gate" } ]
      }
    }
  ]
}
JSON
```

This step mutates **live repository settings** (outward-facing). It MUST be idempotent: detect an
existing ruleset of the same name and update rather than duplicate. It MUST also degrade gracefully —
if `gh` is unauthenticated or lacks admin scope, the step SHALL report what the owner must apply
manually instead of failing the whole change.

### 4. Document the gate

Add a short note (in the workflow comment and/or a brief section in project docs) describing that
`main` is protected by the `ci-gate` required check, how to reproduce CI locally (`npm ci &&
npm run build && npm test` or `just test`), and how an admin would adjust/remove the ruleset.

## Affected Memory

- `build/toolchain`: (modify) Document that CI now gates PRs via a `ci-gate` required status check
  on `main`, in addition to the existing post-merge build/test verification. The toolchain memory
  already covers esbuild + the Node.js test runner + Homebrew distribution; this adds the
  PR-gating CI behavior.

## Impact

- **`.github/workflows/ci.yml`** — trigger block extended (`pull_request` added); new `ci-gate` job;
  header comment updated. No change to the `build-and-test` step list.
- **Repository settings (`sahil87/tu`)** — a new branch ruleset on `main` requiring `ci-gate`.
  Outward-facing, applied via `gh api`. Requires admin token scope.
- **`release.yml`** — *not modified*. Release/tag/Homebrew side effects remain isolated there. The
  release-merge commit still pushes to `main`, which fires the retained `push: [main]` CI trigger.
- **Contributor / agent workflow** — PRs to `main` (including automated ones such as the help-dump
  PR flow) now require a green `ci-gate` before merge. The first PR after the ruleset lands will be
  the de facto verification.
- **No source code (`src/`) impact** — workflow + repo-config only. The CLI behavior, bundle, and
  data model are untouched.
- **Out of scope / dependency**: the known test-suite hermeticity work (backlog item re: `TU_*` env
  leakage, `cli-sync` git fixtures, `rain` flake) is *not* part of this change. Those affect whether
  the suite is reliably green on arbitrary runners; this change wires the gate. If the suite is flaky
  on CI, the gate will surface it — which is the gate working as intended — but fixing flakiness is
  tracked separately.

## Open Questions

- None. All design choices are resolved: trigger structure (PR + push), gate mechanism (`ci-gate`
  job + rulesets requirement), the enforcement API (rulesets, confirmed in `/fab-clarify`), and the
  CI lane shape (single `ubuntu-latest` + Node 20, no matrix, confirmed in `/fab-clarify`).

## Clarifications

### Session 2026-06-04

| # | Question | Resolution |
|---|----------|------------|
| 7 | Rulesets API vs. classic branch-protection to require `ci-gate` | Rulesets API (`POST /repos/sahil87/tu/rulesets`) — user confirmed |
| 8 | Single Node lane vs. version matrix | Single `ubuntu-latest` + Node 20 lane, no matrix — user confirmed |

### Session 2026-06-04 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 4 | Confirmed | — |
| 5 | Confirmed | — |
| 6 | Confirmed | — |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Extend `ci.yml` to trigger on both `pull_request: [main]` and the existing `push: [main]` | User explicitly chose this option in conversation ("Add pull_request, keep push") | S:98 R:85 A:90 D:95 |
| 2 | Certain | Add an explicit aggregating `ci-gate` job as the single required status-check name | User's note directed gating PRs on `ci-gate`; standard pattern for a stable check name | S:95 R:80 A:88 D:90 |
| 3 | Certain | Auto-apply a GitHub ruleset on `main` requiring `ci-gate`, as part of the change | User's note: "Also add the required ruleset that gates PRs on ci-gate" — explicit instruction to apply, not just document | S:95 R:55 A:80 D:90 |
| 4 | Certain | Reuse the existing `build-and-test` job/steps unchanged; only add triggers + gate job | Clarified — user confirmed | S:95 R:80 A:90 D:85 |
| 5 | Certain | Keep `release.yml` untouched; release/Homebrew side effects stay isolated there | Clarified — user confirmed | S:95 R:80 A:88 D:85 |
| 6 | Certain | Ruleset application must be idempotent and degrade gracefully without admin scope | Clarified — user confirmed | S:95 R:65 A:80 D:80 |
| 7 | Certain | Use the modern GitHub **rulesets** API (`POST /repos/sahil87/tu/rulesets`) with a `required_status_checks` rule for `ci-gate`, not the classic branch-protection endpoint | Clarified — user confirmed rulesets (GitHub's current recommendation) | S:95 R:60 A:60 D:50 |
| 8 | Certain | Run `build-and-test` (and `ci-gate`) on a single `ubuntu-latest` + Node 20 lane — no version matrix | Clarified — user confirmed single lane; matches current `ci.yml` pin, no multi-version requirement | S:95 R:75 A:70 D:65 |

8 assumptions (8 certain, 0 confident, 0 tentative, 0 unresolved).
<!-- clarified: gate enforcement uses the rulesets API (required_status_checks → ci-gate), confirmed by user 2026-06-04 -->
<!-- clarified: single ubuntu-latest + Node 20 lane, no matrix, confirmed by user 2026-06-04 -->
