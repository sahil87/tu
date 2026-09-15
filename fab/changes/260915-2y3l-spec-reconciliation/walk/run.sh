#!/usr/bin/env bash
# Binary walk for P1 spec reconciliation. Captures stdout/stderr/exit per cell.
# Never runs: tu sync (live, real HOME), --sync (real HOME), tu update (without --help), init-metrics <url> (real HOME).
set -u
W="${WALK_OUT:-$(cd "$(dirname "$0")" && pwd)}"
OUT="$W/cells"; rm -rf "$OUT"; mkdir -p "$OUT"
IDX="$W/index.tsv"; : > "$IDX"
N=0
TU=/home/linuxbrew/.linuxbrew/bin/tu

# cap <name> <env...> -- <cmd...>   (env entries are KEY=VAL or -u KEY, passed to env)
cap() {
  local name="$1"; shift
  local envargs=()
  while [ "$1" != "--" ]; do envargs+=("$1"); shift; done; shift
  N=$((N+1)); local id; id=$(printf '%03d' "$N")
  local base="$OUT/$id-$name"
  env "${envargs[@]}" "$@" >"$base.out" 2>"$base.err"; local ec=$?
  echo "$ec" > "$base.exit"
  printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$name" "$ec" "$(wc -c <"$base.out")" "$(wc -c <"$base.err")" >> "$IDX"
}

# ---- sandboxes -------------------------------------------------------------
SB="$W/sandbox"; rm -rf "$SB"; mkdir -p "$SB"
# bare repos for sandboxed multi mode
git init -q --bare "$SB/bare-a.git"; git init -q --bare "$SB/bare-b.git"
# single: empty HOME
mkdir -p "$SB/home-single"
# multi: tu.conf pointing at bare-a
mkdir -p "$SB/home-multi/.config/tu"
cat > "$SB/home-multi/.config/tu/tu.conf" <<EOF
version = 2
metrics_repo = $SB/bare-a.git
machine = sandbox-mach
user = sbuser
EOF
# org-only: org.conf only, pointing at bare-b
mkdir -p "$SB/home-org/.config/tu"
cat > "$SB/home-org/.config/tu/org.conf" <<EOF
version = 2
metrics_repo = $SB/bare-b.git
user = orguser
EOF
# org + user override
mkdir -p "$SB/home-orguser/.config/tu"
cp "$SB/home-org/.config/tu/org.conf" "$SB/home-orguser/.config/tu/org.conf"
printf 'version = 2\nuser = personal\n' > "$SB/home-orguser/.config/tu/tu.conf"
# legacy: ~/.tu.conf only
mkdir -p "$SB/home-legacy"
printf 'version = 2\nuser = legacyuser\nmachine = legacymach\n' > "$SB/home-legacy/.tu.conf"
# newer version conf
mkdir -p "$SB/home-vnew/.config/tu"
printf 'version = 9\n' > "$SB/home-vnew/.config/tu/tu.conf"
# mode field + auto_sync falsy + tilde metrics_dir
mkdir -p "$SB/home-modefield/.config/tu"
printf 'version = 2\nmode = multi\nauto_sync = 0\nmetrics_dir = ~/customdir\n' > "$SB/home-modefield/.config/tu/tu.conf"
# reserved user all
mkdir -p "$SB/home-userall/.config/tu"
printf "version = 2\nuser = all\nmetrics_repo = $SB/bare-a.git\n" > "$SB/home-userall/.config/tu/tu.conf"
# bogus repo (auto-clone failure)
mkdir -p "$SB/home-badrepo/.config/tu"
printf 'version = 2\nmetrics_repo = /nonexistent/path/repo.git\n' > "$SB/home-badrepo/.config/tu/tu.conf"
# metrics dir exists but is not a git repo
mkdir -p "$SB/home-notgit/.config/tu" "$SB/home-notgit/.tu/metrics_repo"
printf "version = 2\nmetrics_repo = $SB/bare-a.git\n" > "$SB/home-notgit/.config/tu/tu.conf"


# link the machine's real assistant data dirs into a sandbox HOME so ccusage finds usage
linkdata() { for d in .claude .codex .gemini .copilot .kimi; do [ -e "$HOME/$d" ] && ln -sfn "$HOME/$d" "$1/$d"; done; mkdir -p "$1/.local/share"; [ -e "$HOME/.local/share/opencode" ] && ln -sfn "$HOME/.local/share/opencode" "$1/.local/share/opencode"; return 0; }
for h in "$SB"/home-*; do linkdata "$h"; done
for h in home-single2 home-single3 home-single4; do mkdir -p "$SB/$h"; linkdata "$SB/$h"; done
S=(-u TU_METRICS_REPO NO_COLOR=1 COLUMNS=100)
HS="HOME=$SB/home-single"; HM="HOME=$SB/home-multi"; HO="HOME=$SB/home-org"; HL="HOME=$SB/home-legacy"
REAL=(NO_COLOR=1 COLUMNS=100)   # real HOME, real config (multi mode, read-only observation)

# ---- non-data / toolkit ------------------------------------------------------
cap help            "${S[@]}" $HS -- $TU help
cap help-h          "${S[@]}" $HS -- $TU -h
cap help-long       "${S[@]}" $HS -- $TU --help
cap version         "${S[@]}" $HS -- $TU --version
cap version-V       "${S[@]}" $HS -- $TU -V
cap version-v       "${S[@]}" $HS -- $TU -v
cap help-dump       "${S[@]}" $HS -- $TU help-dump
cap skill           "${S[@]}" $HS -- $TU skill
cap shellinit-bash  "${S[@]}" $HS -- $TU shell-init bash
cap shellinit-zsh   "${S[@]}" $HS -- $TU shell-init zsh
cap shellinit-fish  "${S[@]}" $HS -- $TU shell-init fish
cap shellinit-none  "${S[@]}" $HS -- $TU shell-init
cap shellinit-bogus "${S[@]}" $HS -- $TU shell-init tcsh
cap update-help     "${S[@]}" $HS -- $TU update --help
cap update-h        "${S[@]}" $HS -- $TU update -h
cap cc-help         "${S[@]}" $HS -- $TU cc --help
cap h-help-flag     "${S[@]}" $HS -- $TU h -h
cap bogus-arg       "${S[@]}" $HS -- $TU bogus
cap two-sources     "${S[@]}" $HS -- $TU cc codex
cap nohome-status   -u TU_METRICS_REPO -u HOME NO_COLOR=1 -- $TU status
cap nohome-data     -u TU_METRICS_REPO -u HOME NO_COLOR=1 -- $TU
cap nohome-version  -u TU_METRICS_REPO -u HOME NO_COLOR=1 -- $TU --version
cap nohome-helpdump -u TU_METRICS_REPO -u HOME NO_COLOR=1 -- $TU help-dump
cap nohome-skill    -u TU_METRICS_REPO -u HOME NO_COLOR=1 -- $TU skill
cap nohome-shellinit -u TU_METRICS_REPO -u HOME NO_COLOR=1 -- $TU shell-init zsh
cap nohome-updatehelp -u TU_METRICS_REPO -u HOME NO_COLOR=1 -- $TU update --help
cap emptyhome-status -u TU_METRICS_REPO HOME= NO_COLOR=1 -- $TU status

# ---- single mode (temp HOME) --------------------------------------------------
cap s-status        "${S[@]}" $HS -- $TU status
cap s-snapshot      "${S[@]}" $HS -- $TU
cap s-snapshot-color -u TU_METRICS_REPO -u NO_COLOR COLUMNS=100 $HS -- $TU
cap s-snapshot-nocolorflag -u TU_METRICS_REPO -u NO_COLOR COLUMNS=100 $HS -- $TU --no-color
cap s-cc            "${S[@]}" $HS -- $TU cc
cap s-codex         "${S[@]}" $HS -- $TU codex
cap s-co            "${S[@]}" $HS -- $TU co
cap s-oc            "${S[@]}" $HS -- $TU oc
cap s-gemini        "${S[@]}" $HS -- $TU gemini
cap s-gem           "${S[@]}" $HS -- $TU gem
cap s-copilot       "${S[@]}" $HS -- $TU copilot
cap s-cop           "${S[@]}" $HS -- $TU cop
cap s-kimi          "${S[@]}" $HS -- $TU kimi
cap s-ki            "${S[@]}" $HS -- $TU ki
cap s-all           "${S[@]}" $HS -- $TU all
cap s-d             "${S[@]}" $HS -- $TU d
cap s-daily         "${S[@]}" $HS -- $TU daily
cap s-w             "${S[@]}" $HS -- $TU w
cap s-weekly        "${S[@]}" $HS -- $TU weekly
cap s-m             "${S[@]}" $HS -- $TU m
cap s-monthly       "${S[@]}" $HS -- $TU monthly
cap s-h             "${S[@]}" $HS -- $TU h
cap s-history       "${S[@]}" $HS -- $TU history
cap s-dh            "${S[@]}" $HS -- $TU dh
cap s-wh            "${S[@]}" $HS -- $TU wh
cap s-mh            "${S[@]}" $HS -- $TU mh
cap s-cc-h          "${S[@]}" $HS -- $TU cc h
cap s-cc-wh         "${S[@]}" $HS -- $TU cc wh
cap s-cc-mh         "${S[@]}" $HS -- $TU cc mh
cap s-cc-m          "${S[@]}" $HS -- $TU cc m
cap s-cc-w          "${S[@]}" $HS -- $TU cc w
cap s-h-full        "${S[@]}" $HS -- $TU h --full
cap s-mh-full       "${S[@]}" $HS -- $TU mh --full
cap s-snap-full     "${S[@]}" $HS -- $TU --full
cap s-lb            "${S[@]}" $HS -- $TU lb
cap s-lbh           "${S[@]}" $HS -- $TU lbh
cap s-m-lb          "${S[@]}" $HS -- $TU m lb
cap s-u-other       "${S[@]}" $HS -- $TU -u someone
cap s-u-all         "${S[@]}" $HS -- $TU -u all
cap s-u-missing     "${S[@]}" $HS -- $TU -u
cap s-bymachine     "${S[@]}" $HS -- $TU --by-machine
cap s-cc-h-bymachine "${S[@]}" $HS -- $TU cc h --by-machine
cap s-h-bymachine   "${S[@]}" $HS -- $TU h --by-machine
cap s-since         "${S[@]}" $HS -- $TU h --since 2026-08-01
cap s-since-s       "${S[@]}" $HS -- $TU h -s 20260801
cap s-until         "${S[@]}" $HS -- $TU h --until 2026-08-15
cap s-since-until   "${S[@]}" $HS -- $TU h --since 2026-08-01 --until 2026-08-15
cap s-since-inverted "${S[@]}" $HS -- $TU h --since 2026-08-15 --until 2026-08-01
cap s-since-bad     "${S[@]}" $HS -- $TU h --since 2026/08/01
cap s-since-missing "${S[@]}" $HS -- $TU h --since
cap s-since-impossible "${S[@]}" $HS -- $TU h --since 2026-13-01
cap s-since-snapshot "${S[@]}" $HS -- $TU --since 2026-08-01
cap s-since-mh      "${S[@]}" $HS -- $TU mh --since 2026-08-15
cap s-full-since    "${S[@]}" $HS -- $TU h --full --since 2026-08-01
cap s-metric-tokens "${S[@]}" $HS -- $TU --metric tokens
cap s-metric-cost   "${S[@]}" $HS -- $TU --metric cost
cap s-metric-bad    "${S[@]}" $HS -- $TU --metric bogus
cap s-metric-missing "${S[@]}" $HS -- $TU --metric
cap s-t             "${S[@]}" $HS -- $TU -t
cap s-t-h           "${S[@]}" $HS -- $TU h -t
cap s-t-cc-h        "${S[@]}" $HS -- $TU cc h -t
cap s-t-metric-tokens "${S[@]}" $HS -- $TU -t --metric tokens
cap s-t-metric-cost "${S[@]}" $HS -- $TU -t --metric cost
cap s-top3          "${S[@]}" $HS -- $TU --top 3
cap s-top0          "${S[@]}" $HS -- $TU lb --top 0
cap s-topneg        "${S[@]}" $HS -- $TU lb --top -1
cap s-topabc        "${S[@]}" $HS -- $TU lb --top abc
cap s-topmissing    "${S[@]}" $HS -- $TU lb --top
cap s-json          "${S[@]}" $HS -- $TU --json
cap s-j             "${S[@]}" $HS -- $TU -j
cap s-cc-json       "${S[@]}" $HS -- $TU cc --json
cap s-h-json        "${S[@]}" $HS -- $TU h --json
cap s-cc-h-json     "${S[@]}" $HS -- $TU cc h --json
cap s-mh-json       "${S[@]}" $HS -- $TU mh --json
cap s-csv           "${S[@]}" $HS -- $TU --csv
cap s-cc-csv        "${S[@]}" $HS -- $TU cc --csv
cap s-h-csv         "${S[@]}" $HS -- $TU h --csv
cap s-cc-h-csv      "${S[@]}" $HS -- $TU cc h --csv
cap s-md            "${S[@]}" $HS -- $TU --md
cap s-cc-md         "${S[@]}" $HS -- $TU cc --md
cap s-h-md          "${S[@]}" $HS -- $TU h --md
cap s-cc-h-md       "${S[@]}" $HS -- $TU cc h --md
cap s-h-md-full     "${S[@]}" $HS -- $TU h --md --full
cap s-json-csv      "${S[@]}" $HS -- $TU --json --csv
cap s-json-md       "${S[@]}" $HS -- $TU --json --md
cap s-csv-md        "${S[@]}" $HS -- $TU --csv --md
cap s-json-watch    "${S[@]}" $HS -- $TU --json --watch
cap s-csv-watch     "${S[@]}" $HS -- $TU --csv -w
cap s-md-watch      "${S[@]}" $HS -- $TU --md -w
cap s-json-t        "${S[@]}" $HS -- $TU --json -t
cap s-csv-t-h       "${S[@]}" $HS -- $TU h --csv -t
cap s-bymachine-json "${S[@]}" $HS -- $TU --by-machine --json
cap s-bymachine-csv "${S[@]}" $HS -- $TU --by-machine --csv
cap s-bymachine-md  "${S[@]}" $HS -- $TU --by-machine --md
cap s-cc-h-bymachine-csv "${S[@]}" $HS -- $TU cc h --by-machine --csv
cap s-cc-h-bymachine-md "${S[@]}" $HS -- $TU cc h --by-machine --md
cap s-fresh         "${S[@]}" $HS -- $TU --fresh
cap s-f             "${S[@]}" $HS -- $TU -f
cap s-interval-nowatch "${S[@]}" $HS -- $TU --interval 30
cap s-interval-4    "${S[@]}" $HS -- $TU -w -i 4
cap s-interval-3601 "${S[@]}" $HS -- $TU -w --interval 3601
cap s-interval-abc  "${S[@]}" $HS -- $TU -w -i abc
cap s-interval-missing "${S[@]}" $HS -- $TU -w -i
cap s-norain-nowatch "${S[@]}" $HS -- $TU --no-rain
cap s-dryrun-bare   "${S[@]}" $HS -- $TU --dry-run
cap s-dryrun-cc     "${S[@]}" $HS -- $TU cc --dry-run
cap s-dryrun-sync-flag "${S[@]}" $HS -- $TU cc --sync --dry-run
cap s-sync          "${S[@]}" $HS -- $TU sync
cap s-sync-dryrun   "${S[@]}" $HS -- $TU sync --dry-run
cap s-sync-flag     "${S[@]}" $HS -- $TU --sync
cap s-initmetrics-norepo "${S[@]}" $HS -- $TU init-metrics
cap s-initmetrics-2args "${S[@]}" $HS -- $TU init-metrics a b
cap s-initconf-fresh "${S[@]}" $HS -- $TU init-conf
cap s-initconf-again "${S[@]}" $HS -- $TU init-conf
cap s-status-afterconf "${S[@]}" $HS -- $TU status
cap s-initconf-json "${S[@]}" $HS -- $TU init-conf --json
cap s-status-json   "${S[@]}" $HS -- $TU status --json
cap s-status-fresh  "${S[@]}" $HS -- $TU status --fresh
cap s-status-watch  "${S[@]}" $HS -- $TU status --watch
cap s-help-json     "${S[@]}" $HS -- $TU help --json
cap s-env-metrics-repo -u NO_COLOR NO_COLOR=1 COLUMNS=100 "TU_METRICS_REPO=$SB/bare-b.git" "HOME=$SB/home-single2" -- $TU status
cap s-narrow50      -u TU_METRICS_REPO NO_COLOR=1 COLUMNS=50 $HS -- $TU
cap s-narrow50-h    -u TU_METRICS_REPO NO_COLOR=1 COLUMNS=50 $HS -- $TU h
cap s-narrow50-cc-h -u TU_METRICS_REPO NO_COLOR=1 COLUMNS=50 $HS -- $TU cc h
cap s-width80-h     -u TU_METRICS_REPO NO_COLOR=1 COLUMNS=80 $HS -- $TU h --full
cap s-width80-cc-h  -u TU_METRICS_REPO NO_COLOR=1 COLUMNS=80 $HS -- $TU cc h
cap s-width120-cc-h -u TU_METRICS_REPO NO_COLOR=1 COLUMNS=120 $HS -- $TU cc h
cap s-color-cc-h    -u TU_METRICS_REPO -u NO_COLOR COLUMNS=100 $HS -- $TU cc h
cap s-color-h       -u TU_METRICS_REPO -u NO_COLOR COLUMNS=100 $HS -- $TU h
cap s-color-mh      -u TU_METRICS_REPO -u NO_COLOR COLUMNS=100 $HS -- $TU mh
cap s-color-h-full  -u TU_METRICS_REPO -u NO_COLOR COLUMNS=100 $HS -- $TU h --full

# ---- legacy / org / version / mode-field / reserved / badrepo / notgit --------
cap l-status        "${S[@]}" $HL -- $TU status
cap l-snapshot      "${S[@]}" $HL -- $TU
cap l-json          "${S[@]}" $HL -- $TU --json
cap l-initconf      "${S[@]}" $HL -- $TU init-conf
cap l-status-after  "${S[@]}" $HL -- $TU status
cap o-status        "${S[@]}" $HO -- $TU status
cap o-snapshot      "${S[@]}" $HO -- $TU
cap ou-status       "${S[@]}" "HOME=$SB/home-orguser" -- $TU status
cap vnew-status     "${S[@]}" "HOME=$SB/home-vnew" -- $TU status
cap modefield-status "${S[@]}" "HOME=$SB/home-modefield" -- $TU status
cap userall-snapshot "${S[@]}" "HOME=$SB/home-userall" -- $TU
cap userall-sync    "${S[@]}" "HOME=$SB/home-userall" -- $TU sync
cap badrepo-snapshot "${S[@]}" "HOME=$SB/home-badrepo" -- $TU
cap badrepo-snapshot2 "${S[@]}" "HOME=$SB/home-badrepo" -- $TU
cap badrepo-status  "${S[@]}" "HOME=$SB/home-badrepo" -- $TU status
cap badrepo-sync    "${S[@]}" "HOME=$SB/home-badrepo" -- $TU sync
cap notgit-snapshot "${S[@]}" "HOME=$SB/home-notgit" -- $TU
cap notgit-initmetrics "${S[@]}" "HOME=$SB/home-notgit" -- $TU init-metrics

# ---- sandboxed multi mode (temp HOME + bare-a) --------------------------------
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
cap m-h             "${S[@]}" $HM -- $TU h
cap m-lb            "${S[@]}" $HM -- $TU lb
cap m-lb-json       "${S[@]}" $HM -- $TU lb --json
cap m-lb-csv        "${S[@]}" $HM -- $TU lb --csv
cap m-lb-md         "${S[@]}" $HM -- $TU lb --md
cap m-lbh           "${S[@]}" $HM -- $TU lbh
cap m-lbh-json      "${S[@]}" $HM -- $TU lbh --json
cap m-lbh-csv       "${S[@]}" $HM -- $TU lbh --csv
cap m-lbh-md        "${S[@]}" $HM -- $TU lbh --md
cap m-m-lb          "${S[@]}" $HM -- $TU m lb
cap m-lb-top2       "${S[@]}" $HM -- $TU lb --top 2
cap m-lbh-top1      "${S[@]}" $HM -- $TU lbh --top 1
cap m-lb-bymachine  "${S[@]}" $HM -- $TU lb --by-machine
cap m-lbh-bymachine "${S[@]}" $HM -- $TU lbh --by-machine
cap m-lb-u-all      "${S[@]}" $HM -- $TU lb -u all
cap m-lb-u-name     "${S[@]}" $HM -- $TU lb -u sbuser
cap m-lb-since      "${S[@]}" $HM -- $TU lb --since 2026-09-01 --until 2026-09-10
cap m-lb-until-only "${S[@]}" $HM -- $TU lb --until 2026-09-10
cap m-lb-full       "${S[@]}" $HM -- $TU lb --full
cap m-lbh-full      "${S[@]}" $HM -- $TU lbh --full
cap m-lb-t          "${S[@]}" $HM -- $TU lb -t
cap m-u-all         "${S[@]}" $HM -- $TU -u all
cap m-u-all-h       "${S[@]}" $HM -- $TU h -u all
cap m-u-other       "${S[@]}" $HM -- $TU -u nobody
cap m-u-self        "${S[@]}" $HM -- $TU -u sbuser
cap m-bymachine     "${S[@]}" $HM -- $TU --by-machine
cap m-bymachine-json "${S[@]}" $HM -- $TU --by-machine --json
cap m-bymachine-csv "${S[@]}" $HM -- $TU --by-machine --csv
cap m-bymachine-md  "${S[@]}" $HM -- $TU --by-machine --md
cap m-cc-h-bymachine "${S[@]}" $HM -- $TU cc h --by-machine
cap m-cc-h-bymachine-json "${S[@]}" $HM -- $TU cc h --by-machine --json
cap m-h-bymachine   "${S[@]}" $HM -- $TU h --by-machine
cap m-u-all-bymachine "${S[@]}" $HM -- $TU -u all --by-machine
cap m-u-all-bymachine-json "${S[@]}" $HM -- $TU -u all --by-machine --json
cap m-sync-flag     "${S[@]}" $HM -- $TU --sync
cap m-initmetrics-url "${S[@]}" "HOME=$SB/home-single3" -- $TU init-metrics "$SB/bare-b.git"
cap m-initmetrics-url-status "${S[@]}" "HOME=$SB/home-single3" -- $TU status
cap m-initmetrics-url-conf "${S[@]}" "HOME=$SB/home-single3" -- cat "$SB/home-single3/.config/tu/tu.conf"
cap m-initmetrics-url-again "${S[@]}" "HOME=$SB/home-single3" -- $TU init-metrics "$SB/bare-b.git"
cap m-initmetrics-env-vs-url -u NO_COLOR NO_COLOR=1 COLUMNS=100 "TU_METRICS_REPO=$SB/bare-a.git" "HOME=$SB/home-single4" -- $TU init-metrics "$SB/bare-b.git"
cap m-initmetrics-env-vs-url-remote -u NO_COLOR NO_COLOR=1 "HOME=$SB/home-single4" -- git -C "$SB/home-single4/.tu/metrics_repo" remote get-url origin
cap m-repo-tree     "${S[@]}" $HM -- find "$SB/home-multi/.tu/metrics_repo" -path '*/.git' -prune -o -type f -print
cap m-repo-log      "${S[@]}" $HM -- git -C "$SB/home-multi/.tu/metrics_repo" log --format='%s'
cap m-dayfile       "${S[@]}" $HM -- sh -c "cat \$(find $SB/home-multi/.tu/metrics_repo -name 'cc-*.jsonl' | sort | tail -1)"
cap m-tuhome-tree   "${S[@]}" $HM -- find "$SB/home-multi/.tu" -maxdepth 2 -not -path '*/metrics_repo/*'

# ---- real config (multi, read-only observation; no sync/--sync) ---------------
cap r-status        "${REAL[@]}" -- $TU status
cap r-snapshot      "${REAL[@]}" -- $TU
cap r-snapshot-json "${REAL[@]}" -- $TU --json
cap r-h             "${REAL[@]}" -- $TU h
cap r-h-json        "${REAL[@]}" -- $TU h --json
cap r-h-csv         "${REAL[@]}" -- $TU h --csv
cap r-h-md          "${REAL[@]}" -- $TU h --md
cap r-mh            "${REAL[@]}" -- $TU mh
cap r-wh            "${REAL[@]}" -- $TU wh
cap r-cc-h          "${REAL[@]}" -- $TU cc h
cap r-cc-mh         "${REAL[@]}" -- $TU cc mh
cap r-kimi-h        "${REAL[@]}" -- $TU kimi h
cap r-lb            "${REAL[@]}" -- $TU lb
cap r-m-lb          "${REAL[@]}" -- $TU m lb
cap r-m-lb-json     "${REAL[@]}" -- $TU m lb --json
cap r-m-lb-csv      "${REAL[@]}" -- $TU m lb --csv
cap r-m-lb-md       "${REAL[@]}" -- $TU m lb --md
cap r-m-lb-top2     "${REAL[@]}" -- $TU m lb --top 2
cap r-m-lb-t        "${REAL[@]}" -- $TU m lb -t
cap r-m-lb-bymachine "${REAL[@]}" -- $TU m lb --by-machine
cap r-m-lb-bymachine-json "${REAL[@]}" -- $TU m lb --by-machine --json
cap r-m-lb-bymachine-csv "${REAL[@]}" -- $TU m lb --by-machine --csv
cap r-lbh           "${REAL[@]}" -- $TU lbh
cap r-m-lbh         "${REAL[@]}" -- $TU m lbh
cap r-m-lbh-top2    "${REAL[@]}" -- $TU m lbh --top 2
cap r-m-lbh-json    "${REAL[@]}" -- $TU m lbh --json
cap r-m-lbh-csv     "${REAL[@]}" -- $TU m lbh --csv
cap r-m-lbh-md      "${REAL[@]}" -- $TU m lbh --md
cap r-m-lbh-t       "${REAL[@]}" -- $TU m lbh -t
cap r-u-all         "${REAL[@]}" -- $TU -u all
cap r-u-all-m       "${REAL[@]}" -- $TU m -u all
cap r-u-all-h       "${REAL[@]}" -- $TU h -u all
cap r-u-all-bymachine "${REAL[@]}" -- $TU -u all --by-machine
cap r-u-all-bymachine-json "${REAL[@]}" -- $TU -u all --by-machine --json
cap r-u-all-bymachine-md "${REAL[@]}" -- $TU -u all --by-machine --md
cap r-u-all-bymachine-h "${REAL[@]}" -- $TU cc h -u all --by-machine
cap r-bymachine     "${REAL[@]}" -- $TU --by-machine
cap r-bymachine-m   "${REAL[@]}" -- $TU m --by-machine
cap r-bymachine-json "${REAL[@]}" -- $TU --by-machine --json
cap r-bymachine-csv "${REAL[@]}" -- $TU --by-machine --csv
cap r-bymachine-md  "${REAL[@]}" -- $TU --by-machine --md
cap r-cc-h-bymachine "${REAL[@]}" -- $TU cc h --by-machine
cap r-cc-h-bymachine-t "${REAL[@]}" -- $TU cc h --by-machine -t
cap r-cc-h-bymachine-json "${REAL[@]}" -- $TU cc h --by-machine --json
cap r-cc-h-bymachine-csv "${REAL[@]}" -- $TU cc h --by-machine --csv
cap r-cc-h-bymachine-md "${REAL[@]}" -- $TU cc h --by-machine --md
cap r-sync-dryrun   "${REAL[@]}" -- $TU sync --dry-run
cap r-h-t           "${REAL[@]}" -- $TU h -t
cap r-h-color       -u NO_COLOR COLUMNS=100 -- $TU h
cap r-mh-color      -u NO_COLOR COLUMNS=100 -- $TU mh
cap r-narrow50      NO_COLOR=1 COLUMNS=50 -- $TU
cap r-narrow50-h    NO_COLOR=1 COLUMNS=50 -- $TU h
cap r-narrow59-h    NO_COLOR=1 COLUMNS=59 -- $TU h
cap r-width60-h     NO_COLOR=1 COLUMNS=60 -- $TU h
cap r-width80-h     NO_COLOR=1 COLUMNS=80 -- $TU h
cap r-width96-h-full NO_COLOR=1 COLUMNS=96 -- $TU h --full
cap r-width80-cc-h  NO_COLOR=1 COLUMNS=80 -- $TU cc h
cap r-u-other       "${REAL[@]}" -- $TU -u akshay
cap r-u-other-h     "${REAL[@]}" -- $TU h -u akshay
cap r-u-self        "${REAL[@]}" -- $TU -u sahil
cap r-u-all-t       "${REAL[@]}" -- $TU -u all -t
cap r-lb-since      "${REAL[@]}" -- $TU lb --since 2026-09-01 --until 2026-09-10
cap r-lb-until-only "${REAL[@]}" -- $TU lb --until 2026-09-10
cap r-lb-full       "${REAL[@]}" -- $TU lb --full
cap r-lbh-since     "${REAL[@]}" -- $TU lbh --since 2026-09-01
cap r-lb-u-other    "${REAL[@]}" -- $TU m lb -u akshay
cap r-lb-top-md     "${REAL[@]}" -- $TU m lb --top 2 --md
cap r-lb-top-csv    "${REAL[@]}" -- $TU m lb --top 2 --csv
cap r-lb-top-json   "${REAL[@]}" -- $TU m lb --top 2 --json
cap r-lbh-top-csv   "${REAL[@]}" -- $TU m lbh --top 2 --csv
cap r-cache-ls      -- ls -la "$HOME/.tu/cache"
cap r-tuhome-ls     -- ls -la "$HOME/.tu"
cap r-repo-tree-sample -- sh -c "find $HOME/.tu/metrics_repo -maxdepth 3 -type d | head -30"
cap r-listusers     -- ls "$HOME/.tu/metrics_repo"

echo "cells: $N"
