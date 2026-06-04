#!/usr/bin/env bash
set -euo pipefail

# scripts/ci-gate-ruleset.sh — Apply (or preview) the branch ruleset that gates
# PR merges on the `ci-gate` required status check for `main`.
#
# WHAT THIS DOES (with --apply):
#   Creates or updates a GitHub *ruleset* on the repo that requires the
#   `ci-gate` status check (the aggregating job in .github/workflows/ci.yml)
#   to pass before a pull request targeting `main` can be merged.
#
# !! THIS MUTATES LIVE REPOSITORY SETTINGS !!
#   It calls the GitHub rulesets API and needs a `gh` token with ADMIN scope on
#   the repo. By default the script runs in DRY-RUN mode and changes nothing —
#   it only prints what it would do. Pass --apply to perform the mutation.
#
# It is IDEMPOTENT: if a ruleset of the same name already exists it is UPDATED
#   (PUT) in place rather than creating a duplicate.
#
# It DEGRADES GRACEFULLY: if `gh` is missing, unauthenticated, or lacks admin
#   scope, it prints the manual steps and exits 0 (it never crashes the caller).
#
# Usage:
#   scripts/ci-gate-ruleset.sh            # dry-run: print the payload + plan
#   scripts/ci-gate-ruleset.sh --apply    # create/update the ruleset (needs admin)
#   scripts/ci-gate-ruleset.sh -h         # help

# Resolve the target repo from the local clone's `origin` remote so the ruleset
# is never applied to the wrong repository (e.g. from a fork or a renamed
# clone). Prefer `gh` (authoritative for the GitHub repo), fall back to parsing
# the origin URL, and finally to the canonical default. The trailing `|| true`
# keeps `set -e` from aborting when neither source resolves.
REPO_DEFAULT="sahil87/tu"
REPO="$(gh repo view --json nameWithOwner -q '.nameWithOwner' 2>/dev/null || true)"
if [ -z "$REPO" ]; then
  origin_url="$(git config --get remote.origin.url 2>/dev/null || true)"
  # Strip protocol/host and a trailing .git → owner/repo (handles git@ and https forms).
  REPO="$(printf '%s' "$origin_url" | sed -E 's#^[^:]+://[^/]+/##; s#^[^:]+:##; s#\.git$##')"
fi
[ -z "$REPO" ] && REPO="$REPO_DEFAULT"

RULESET_NAME="Require CI gate on main"
CHECK_CONTEXT="ci-gate"
TARGET_REF="refs/heads/main"

APPLY=0

usage() {
  cat <<EOF
Usage: ci-gate-ruleset.sh [--apply]

Create or update the GitHub branch ruleset that requires the '$CHECK_CONTEXT'
status check on '$TARGET_REF' of '$REPO' before a PR can be merged.

  (no args)   Dry-run. Print the ruleset payload and what would happen. No mutation.
  --apply     Perform the create/update via 'gh api'. Requires a gh token with
              admin scope on the repo. Idempotent (updates an existing ruleset
              of the same name instead of duplicating).
  -h, --help  Show this help.

This script mutates LIVE repository settings when run with --apply.
EOF
}

# ── Parse arguments ──────────────────────────────────────────────────
for arg in "$@"; do
  case "$arg" in
    --apply) APPLY=1 ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "ERROR: Unknown argument '$arg'." >&2
      echo "" >&2
      usage >&2
      exit 1
      ;;
  esac
done

# ── The ruleset payload (single source of truth) ─────────────────────
# strict_required_status_checks_policy=false: do not require the branch to be
# up to date with main before merging (matches the intake design).
read -r -d '' PAYLOAD <<JSON || true
{
  "name": "$RULESET_NAME",
  "target": "branch",
  "enforcement": "active",
  "conditions": { "ref_name": { "include": ["$TARGET_REF"], "exclude": [] } },
  "rules": [
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": false,
        "required_status_checks": [ { "context": "$CHECK_CONTEXT" } ]
      }
    }
  ]
}
JSON

manual_instructions() {
  cat <<EOF

Apply it manually (needs admin on $REPO):

  1. Authenticate gh with an admin-scoped token:  gh auth login
  2. Re-run:  scripts/ci-gate-ruleset.sh --apply

  Or via the GitHub UI:
    Settings → Rules → Rulesets → New branch ruleset
      Name:        $RULESET_NAME
      Enforcement: Active
      Target:      branch  →  include  $TARGET_REF
      Rules:       Require status checks to pass  →  add check '$CHECK_CONTEXT'

  Or call the API directly with the payload below:
    gh api -X POST repos/$REPO/rulesets --input - <<'PAYLOAD'
$PAYLOAD
PAYLOAD
EOF
}

# ── Dry-run: print and exit, no gh needed ────────────────────────────
if [ "$APPLY" -ne 1 ]; then
  echo "DRY-RUN — no changes made. Re-run with --apply to enforce."
  echo ""
  echo "Ruleset '$RULESET_NAME' on $REPO would require check '$CHECK_CONTEXT' for $TARGET_REF."
  echo ""
  echo "Payload:"
  echo "$PAYLOAD"
  exit 0
fi

# ── Graceful degradation: gh must exist and be authenticated ─────────
if ! command -v gh >/dev/null 2>&1; then
  echo "WARN: 'gh' CLI not found — cannot apply the ruleset automatically." >&2
  manual_instructions
  exit 0
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "WARN: 'gh' is not authenticated — cannot apply the ruleset automatically." >&2
  manual_instructions
  exit 0
fi

# ── Idempotency: find an existing ruleset of the same name ───────────
# A missing/forbidden endpoint (no admin scope) makes this fail; treat any
# failure as "cannot read rulesets" and degrade gracefully.
existing_id=""
if existing_id="$(gh api "repos/$REPO/rulesets" \
    --jq ".[] | select(.name == \"$RULESET_NAME\") | .id" 2>/dev/null)"; then
  existing_id="$(printf '%s' "$existing_id" | head -1)"
else
  echo "WARN: could not list rulesets on $REPO (missing admin scope?) — cannot apply automatically." >&2
  manual_instructions
  exit 0
fi

# ── Create or update ─────────────────────────────────────────────────
if [ -n "$existing_id" ]; then
  echo "Updating existing ruleset '$RULESET_NAME' (id=$existing_id) on $REPO…"
  if printf '%s' "$PAYLOAD" | gh api -X PUT "repos/$REPO/rulesets/$existing_id" --input - >/dev/null; then
    echo "Ruleset updated. '$CHECK_CONTEXT' is required for $TARGET_REF."
  else
    echo "WARN: update failed (likely missing admin scope)." >&2
    manual_instructions
    exit 0
  fi
else
  echo "Creating ruleset '$RULESET_NAME' on $REPO…"
  if printf '%s' "$PAYLOAD" | gh api -X POST "repos/$REPO/rulesets" --input - >/dev/null; then
    echo "Ruleset created. '$CHECK_CONTEXT' is now required for $TARGET_REF."
  else
    echo "WARN: create failed (likely missing admin scope)." >&2
    manual_instructions
    exit 0
  fi
fi
