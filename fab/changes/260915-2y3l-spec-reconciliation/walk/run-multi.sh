#!/usr/bin/env bash
# Re-run of the sandboxed multi-mode cells against SEEDED bare repos (initial commit on main, like a GitHub-created repo).
set -u
W="${WALK_OUT:-$(cd "$(dirname "$0")" && pwd)}"
OUT="$W/cells-multi"; rm -rf "$OUT"; mkdir -p "$OUT"
IDX="$W/index-multi.tsv"; : > "$IDX"; N=0
TU=/home/linuxbrew/.linuxbrew/bin/tu
cap() { local name="$1"; shift; local envargs=(); while [ "$1" != "--" ]; do envargs+=("$1"); shift; done; shift
  N=$((N+1)); local id; id=$(printf '%03d' "$N"); local base="$OUT/$id-$name"
  env "${envargs[@]}" "$@" >"$base.out" 2>"$base.err"; local ec=$?; echo "$ec" > "$base.exit"
  printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$name" "$ec" "$(wc -c <"$base.out")" "$(wc -c <"$base.err")" >> "$IDX"; }
SB="$W/sandbox2"; rm -rf "$SB"; mkdir -p "$SB"
seed() { git init -q --bare -b main "$1"; local t; t=$(mktemp -d); git -C "$t" init -q -b main; echo "# metrics" > "$t/README.md"; git -C "$t" add README.md; git -C "$t" -c user.name=seed -c user.email=seed@x commit -qm "init"; git -C "$t" push -q "$1" main; rm -rf "$t"; }
seed "$SB/bare-a.git"; seed "$SB/bare-b.git"
# a second "other user" and "other machine" pre-populated in bare-a so lb/-u all have data
T=$(mktemp -d); git clone -q "$SB/bare-a.git" "$T"; mkdir -p "$T/otheruser/2026/othermach" "$T/sbuser/2026/othermach" "$T/docs"
echo '{"label":"2026-09-15","totalCost":12.5,"inputTokens":100,"outputTokens":200,"cacheCreationTokens":300,"cacheReadTokens":400,"totalTokens":1000}' > "$T/otheruser/2026/othermach/cc-2026-09-15.jsonl"
echo '{"label":"2026-09-16","totalCost":7.25,"inputTokens":10,"outputTokens":20,"cacheCreationTokens":30,"cacheReadTokens":40,"totalTokens":100}' > "$T/otheruser/2026/othermach/cc-2026-09-16.jsonl"
echo '{"label":"2026-09-16","totalCost":1.5,"inputTokens":1,"outputTokens":2,"cacheCreationTokens":3,"cacheReadTokens":4,"totalTokens":10}' > "$T/otheruser/2026/othermach/codex-2026-09-16.jsonl"
echo '{"label":"2026-09-16","totalCost":999.99,"inputTokens":1,"outputTokens":2,"cacheCreationTokens":3,"cacheReadTokens":4,"totalTokens":10}' > "$T/sbuser/2026/othermach/cc-2026-09-16.jsonl"
mkdir -p "$T/sbuser/2026/sandbox-mach"; { echo '{"label":"2026-09-14","totalCost":3.0,"inputTokens":1,"outputTokens":2,"cacheCreationTokens":3,"cacheReadTokens":4,"totalTokens":10}' > "$T/sbuser/2026/sandbox-mach/cc-2026-09-14.jsonl"; }
echo "doc" > "$T/docs/README.md"
git -C "$T" add -A; git -C "$T" -c user.name=seed -c user.email=seed@x commit -qm "seed data"; git -C "$T" push -q; rm -rf "$T"
mkdir -p "$SB/home-multi/.config/tu"
printf "version = 2\nmetrics_repo = $SB/bare-a.git\nmachine = sandbox-mach\nuser = sbuser\n" > "$SB/home-multi/.config/tu/tu.conf"

# link the machine's real assistant data dirs into a sandbox HOME so ccusage finds usage
linkdata() { for d in .claude .codex .gemini .copilot .kimi; do [ -e "$HOME/$d" ] && ln -sfn "$HOME/$d" "$1/$d"; done; mkdir -p "$1/.local/share"; [ -e "$HOME/.local/share/opencode" ] && ln -sfn "$HOME/.local/share/opencode" "$1/.local/share/opencode"; return 0; }
for h in home-multi home-s3 home-s4 home-s5; do mkdir -p "$SB/$h"; linkdata "$SB/$h"; done
S=(-u TU_METRICS_REPO NO_COLOR=1 COLUMNS=100); HM="HOME=$SB/home-multi"
cap m-status-preclone "${S[@]}" $HM -- $TU status
cap m-snapshot-autoclone "${S[@]}" $HM -- $TU
cap m-status        "${S[@]}" $HM -- $TU status
cap m-initmetrics-again "${S[@]}" $HM -- $TU init-metrics
cap m-sync-dryrun   "${S[@]}" $HM -- $TU sync --dry-run
cap m-sync          "${S[@]}" $HM -- $TU sync
cap m-sync-again    "${S[@]}" $HM -- $TU sync
cap m-sync-dryrun-after "${S[@]}" $HM -- $TU sync --dry-run
cap m-sync-json     "${S[@]}" $HM -- $TU sync --json
cap m-status-after  "${S[@]}" $HM -- $TU status
cap m-snapshot      "${S[@]}" $HM -- $TU
cap m-snapshot-json "${S[@]}" $HM -- $TU --json
cap m-h             "${S[@]}" $HM -- $TU h
cap m-cc-h          "${S[@]}" $HM -- $TU cc h
cap m-lb            "${S[@]}" $HM -- $TU lb
cap m-lb-json       "${S[@]}" $HM -- $TU lb --json
cap m-lb-csv        "${S[@]}" $HM -- $TU lb --csv
cap m-lb-md         "${S[@]}" $HM -- $TU lb --md
cap m-lbh           "${S[@]}" $HM -- $TU lbh
cap m-lbh-json      "${S[@]}" $HM -- $TU lbh --json
cap m-lbh-csv       "${S[@]}" $HM -- $TU lbh --csv
cap m-lbh-md        "${S[@]}" $HM -- $TU lbh --md
cap m-m-lb          "${S[@]}" $HM -- $TU m lb
cap m-w-lb          "${S[@]}" $HM -- $TU w lb
cap m-lb-top1       "${S[@]}" $HM -- $TU lb --top 1
cap m-lbh-top1      "${S[@]}" $HM -- $TU lbh --top 1
cap m-lb-bymachine  "${S[@]}" $HM -- $TU lb --by-machine
cap m-lb-bymachine-json "${S[@]}" $HM -- $TU lb --by-machine --json
cap m-lb-bymachine-csv "${S[@]}" $HM -- $TU lb --by-machine --csv
cap m-lb-bymachine-md "${S[@]}" $HM -- $TU lb --by-machine --md
cap m-lbh-bymachine "${S[@]}" $HM -- $TU lbh --by-machine
cap m-lb-u-all      "${S[@]}" $HM -- $TU lb -u all
cap m-lb-u-other    "${S[@]}" $HM -- $TU lb -u otheruser
cap m-lb-since      "${S[@]}" $HM -- $TU lb --since 2026-09-15 --until 2026-09-16
cap m-lb-until-only "${S[@]}" $HM -- $TU lb --until 2026-09-16
cap m-lb-t          "${S[@]}" $HM -- $TU lb -t
cap m-lb-cc         "${S[@]}" $HM -- $TU cc lb
cap m-lb-codex      "${S[@]}" $HM -- $TU codex lb
cap m-u-all         "${S[@]}" $HM -- $TU -u all
cap m-u-all-json    "${S[@]}" $HM -- $TU -u all --json
cap m-u-all-h       "${S[@]}" $HM -- $TU h -u all
cap m-u-other       "${S[@]}" $HM -- $TU -u otheruser
cap m-u-other-h     "${S[@]}" $HM -- $TU cc h -u otheruser
cap m-u-nobody      "${S[@]}" $HM -- $TU -u nobody
cap m-u-self        "${S[@]}" $HM -- $TU -u sbuser
cap m-bymachine     "${S[@]}" $HM -- $TU --by-machine
cap m-bymachine-json "${S[@]}" $HM -- $TU --by-machine --json
cap m-bymachine-csv "${S[@]}" $HM -- $TU --by-machine --csv
cap m-bymachine-md  "${S[@]}" $HM -- $TU --by-machine --md
cap m-cc-h-bymachine "${S[@]}" $HM -- $TU cc h --by-machine
cap m-cc-h-bymachine-json "${S[@]}" $HM -- $TU cc h --by-machine --json
cap m-cc-h-bymachine-csv "${S[@]}" $HM -- $TU cc h --by-machine --csv
cap m-cc-h-bymachine-md "${S[@]}" $HM -- $TU cc h --by-machine --md
cap m-u-all-bymachine "${S[@]}" $HM -- $TU -u all --by-machine
cap m-u-all-bymachine-json "${S[@]}" $HM -- $TU -u all --by-machine --json
cap m-u-all-bymachine-csv "${S[@]}" $HM -- $TU -u all --by-machine --csv
cap m-u-all-bymachine-md "${S[@]}" $HM -- $TU -u all --by-machine --md
cap m-u-other-bymachine "${S[@]}" $HM -- $TU -u otheruser --by-machine
cap m-sync-flag     "${S[@]}" $HM -- $TU --sync
cap m-sync-flag-h   "${S[@]}" $HM -- $TU h --sync
cap m-initmetrics-url "${S[@]}" "HOME=$SB/home-s3" -- $TU init-metrics "$SB/bare-b.git"
cap m-initmetrics-url-status "${S[@]}" "HOME=$SB/home-s3" -- $TU status
cap m-initmetrics-url-conf "${S[@]}" "HOME=$SB/home-s3" -- cat "$SB/home-s3/.config/tu/tu.conf"
cap m-initmetrics-url-again "${S[@]}" "HOME=$SB/home-s3" -- $TU init-metrics "$SB/bare-b.git"
cap m-initmetrics-env-vs-url -u NO_COLOR NO_COLOR=1 COLUMNS=100 "TU_METRICS_REPO=$SB/bare-a.git" "HOME=$SB/home-s4" -- $TU init-metrics "$SB/bare-b.git"
cap m-initmetrics-env-vs-url-remote -- git -C "$SB/home-s4/.tu/metrics_repo" remote get-url origin
cap m-initmetrics-legacy "${S[@]}" "HOME=$SB/home-s5" -- sh -c "printf 'version = 2\nuser = legacyu\n' > $SB/home-s5/.tu.conf 2>/dev/null || { mkdir -p $SB/home-s5; printf 'version = 2\nuser = legacyu\n' > $SB/home-s5/.tu.conf; }; $TU init-metrics $SB/bare-b.git; cat $SB/home-s5/.config/tu/tu.conf"
cap m-repo-tree     -- find "$SB/home-multi/.tu/metrics_repo" -path '*/.git' -prune -o -type f -print
cap m-repo-log      -- git -C "$SB/home-multi/.tu/metrics_repo" log --format='%s'
cap m-dayfile       -- sh -c "for f in \$(find $SB/home-multi/.tu/metrics_repo/sbuser/2026/sandbox-mach -name '*.jsonl' | sort | tail -3); do echo \"== \$f\"; cat \$f; echo; done"
cap m-tuhome-tree   -- find "$SB/home-multi/.tu" -maxdepth 2 -not -path '*/metrics_repo/*'
cap m-lastsync      -- cat "$SB/home-multi/.tu/.last-sync"
# never-shrink: write a bigger snapshot for today into the repo for sandbox-mach cc, then fetch again and see the max-merge
BIG="$SB/home-multi/.tu/metrics_repo/sbuser/2026/sandbox-mach/cc-$(date +%F).jsonl"
cap m-dayfile-before -- cat "$BIG"
cap m-inflate -- sh -c "python3 - <<PY
import json,sys
p='$BIG'; d=json.load(open(p)); d['totalCost']=d['totalCost']*1000+5000; d['inputTokens']+=123456789; d['totalTokens']+=123456789; open(p,'w').write(json.dumps(d))
PY
cat '$BIG'"
cap m-snapshot-after-inflate "${S[@]}" $HM -- $TU --fresh
cap m-dayfile-after-fetch -- cat "$BIG"
cap m-sync-dryrun-shrink "${S[@]}" $HM -- $TU sync --dry-run
cap m-bymachine-after-inflate "${S[@]}" $HM -- $TU --by-machine
echo "cells: $N"
