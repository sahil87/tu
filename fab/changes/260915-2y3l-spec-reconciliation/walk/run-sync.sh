#!/usr/bin/env bash
set -u
W="${WALK_OUT:-$(cd "$(dirname "$0")" && pwd)}"; OUT="$W/cells-sync"; rm -rf "$OUT"; mkdir -p "$OUT"; IDX="$W/index-sync.tsv"; : > "$IDX"; N=0
TU=/home/linuxbrew/.linuxbrew/bin/tu; SB="$W/sandbox2"; HM="HOME=$SB/home-multi"
# Prerequisite: run-multi.sh has run under the same WALK_OUT (it creates sandbox2/, the seeded bare repos, and home-multi).
[ -d "$SB/home-multi/.tu/metrics_repo" ] && [ -d "$SB/bare-a.git" ] || { echo "run-sync.sh: sandbox2 missing — run run-multi.sh first (same WALK_OUT)" >&2; exit 1; }
# Writes: index-sync.tsv (committed as index-sync-sandbox.tsv).
cap() { local name="$1"; shift; local envargs=(); while [ "$1" != "--" ]; do envargs+=("$1"); shift; done; shift
  N=$((N+1)); local id; id=$(printf '%03d' "$N"); local base="$OUT/$id-$name"
  env "${envargs[@]}" "$@" >"$base.out" 2>"$base.err"; local ec=$?; echo "$ec" > "$base.exit"
  printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$name" "$ec" "$(wc -c <"$base.out")" "$(wc -c <"$base.err")" >> "$IDX"; }
# git identity inside the sandbox HOME (a real user has one in ~/.gitconfig)
printf '[user]\n\tname = Sandbox\n\temail = sandbox@example.invalid\n' > "$SB/home-multi/.gitconfig"
S=(-u TU_METRICS_REPO NO_COLOR=1 COLUMNS=100)
# restore the inflated day file to what the live fetch would write? leave as is: it exercises never-shrink on the real sync
cap sync-dryrun     "${S[@]}" $HM -- $TU sync --dry-run
cap sync            "${S[@]}" $HM -- $TU sync
cap sync-again      "${S[@]}" $HM -- $TU sync
cap sync-dryrun-after "${S[@]}" $HM -- $TU sync --dry-run
cap sync-json       "${S[@]}" $HM -- $TU sync --json
cap status-after    "${S[@]}" $HM -- $TU status
cap lastsync        -- cat "$SB/home-multi/.tu/.last-sync"
cap repo-log        -- git -C "$SB/home-multi/.tu/metrics_repo" log --format='%s%n%an <%ae>' -3
cap repo-status     -- git -C "$SB/home-multi/.tu/metrics_repo" status --short
cap bare-log        -- git -C "$SB/bare-a.git" log --format='%s' -3
cap dayfile-today   -- cat "$SB/home-multi/.tu/metrics_repo/sbuser/2026/sandbox-mach/cc-$(date +%F).jsonl"
cap lb-after-sync   "${S[@]}" $HM -- $TU lb
# auto-sync probe: remove .last-sync, run data commands, see whether any sync happens
cap rm-lastsync     -- rm -f "$SB/home-multi/.tu/.last-sync"
cap data-after-rm   "${S[@]}" $HM -- $TU --fresh
cap lastsync-after-data -- sh -c "ls -la $SB/home-multi/.tu/.last-sync 2>&1; git -C $SB/home-multi/.tu/metrics_repo status --short | head -3"
# stale .last-sync (4h old) then data command
cap touch-stale     -- sh -c "date -u -d '4 hours ago' +%Y-%m-%dT%H:%M:%S.000Z > $SB/home-multi/.tu/.last-sync; cat $SB/home-multi/.tu/.last-sync"
cap data-when-stale "${S[@]}" $HM -- $TU h
cap lastsync-after-stale -- cat "$SB/home-multi/.tu/.last-sync"
cap status-stale    "${S[@]}" $HM -- $TU status
# auto_sync = false in conf: status line + sync behavior
cap autosync-off-status "${S[@]}" "HOME=$SB/home-multi" -- sh -c "printf 'auto_sync = false\n' >> $SB/home-multi/.config/tu/tu.conf; $TU status; $TU sync; echo exit=\$?"
# remote divergence: another machine pushes, then we sync (pull --rebase path)
T=$(mktemp -d); git clone -q "$SB/bare-a.git" "$T"; mkdir -p "$T/otheruser/2026/othermach"; echo '{"label":"2026-09-16","totalCost":9.5,"inputTokens":1,"outputTokens":2,"cacheCreationTokens":3,"cacheReadTokens":4,"totalTokens":10}' > "$T/otheruser/2026/othermach/cc-2026-09-16.jsonl"; git -C "$T" add -A; git -C "$T" -c user.name=o -c user.email=o@x commit -qm "# otheruser: update 2026-09-16"; git -C "$T" push -q; rm -rf "$T"
cap sync-after-remote-change "${S[@]}" $HM -- $TU sync
cap repo-log-after  -- git -C "$SB/home-multi/.tu/metrics_repo" log --format='%s' -4
cap lb-after-remote "${S[@]}" $HM -- $TU lb
# conflict: both sides change the same otheruser file? tu only writes its own user dir, so conflicts need same user/machine from two clones — simulate a same-file conflict
T=$(mktemp -d); git clone -q "$SB/bare-a.git" "$T"; echo '{"label":"2026-09-15","totalCost":99999,"inputTokens":1,"outputTokens":2,"cacheCreationTokens":3,"cacheReadTokens":4,"totalTokens":10}' > "$T/sbuser/2026/sandbox-mach/cc-2026-09-15.jsonl"; git -C "$T" add -A; git -C "$T" -c user.name=o -c user.email=o@x commit -qm "conflicting"; git -C "$T" push -q; rm -rf "$T"
cap sync-conflict   "${S[@]}" $HM -- sh -c "$TU --fresh >/dev/null; $TU sync; echo exit=\$?; git -C $SB/home-multi/.tu/metrics_repo status | head -3"
echo "cells: $N"
