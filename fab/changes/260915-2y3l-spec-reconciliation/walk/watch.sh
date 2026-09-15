#!/usr/bin/env bash
# Watch-mode frame capture via an isolated tmux server. Uses the real config (multi mode): own-user data commands
# write this machine's own day-files into the local metrics clone (never sync/push); use a sandbox HOME to avoid that.
set -u
W="${WALK_OUT:-$(cd "$(dirname "$0")" && pwd)}"
OUT="$W/watch"; rm -rf "$OUT"; mkdir -p "$OUT"
TU=/home/linuxbrew/.linuxbrew/bin/tu
L=walkwatch

# frames <name> <cols> <rows> <cmd...>
frames() {
  local name="$1" cols="$2" rows="$3"; shift 3
  tmux -L $L kill-server 2>/dev/null
  local cmd; cmd=$(printf '%q ' "$@")   # preserve argument boundaries (e.g. sh -c '...')
  tmux -L $L new-session -d -s w -x "$cols" -y "$rows" "env NO_COLOR=1 $cmd ; echo __EXIT__=\$? ; sleep 30"
  sleep 1.2; tmux -L $L capture-pane -p -t w > "$OUT/$name-0-skeleton.txt"
  sleep 4;   tmux -L $L capture-pane -p -t w > "$OUT/$name-1-first.txt"
  sleep 9;   tmux -L $L capture-pane -p -t w > "$OUT/$name-2-second.txt"
  tmux -L $L send-keys -t w Enter; sleep 0.3
  tmux -L $L capture-pane -p -t w > "$OUT/$name-3-refreshing.txt"
  sleep 3
  tmux -L $L send-keys -t w q; sleep 1.5
  tmux -L $L capture-pane -p -t w -S -200 > "$OUT/$name-4-afterquit.txt"
  tmux -L $L kill-server 2>/dev/null
}

frames snap-100x30   100 30 $TU -w -i 5
frames snap-100x30-t 100 30 $TU -w -i 5 -t
frames pivot-100x30  100 30 $TU h -w -i 5
frames cc-h-100x30   100 30 $TU cc h -w -i 5
frames lb-100x30     100 30 $TU m lb -w -i 5
frames lbh-100x30    100 30 $TU m lbh -w -i 5
frames snap-50x20    50 20 $TU -w -i 5
frames pivot-59x20   59 20 $TU h -w -i 5
frames snap-100x12   100 12 $TU -w -i 5 --no-rain
frames snap-100x30-norain 100 30 $TU -w -i 5 --no-rain
frames snap-color    100 30 sh -c "unset NO_COLOR; $TU -w -i 5"

# colored capture of one frame with escapes
tmux -L $L kill-server 2>/dev/null
tmux -L $L new-session -d -s w -x 100 -y 30 "$TU -w -i 5; sleep 30"
sleep 6; tmux -L $L capture-pane -p -e -t w > "$OUT/snap-color-escapes.txt"
tmux -L $L send-keys -t w C-c; sleep 1.5
tmux -L $L capture-pane -p -t w -S -100 > "$OUT/snap-ctrlc-afterquit.txt"
tmux -L $L kill-server 2>/dev/null

# resize behavior: start at 100, shrink to 50
tmux -L $L new-session -d -s w -x 100 -y 30 "env NO_COLOR=1 $TU -w -i 5; sleep 30"
sleep 6; tmux -L $L resize-window -t w -x 50 -y 20 2>/dev/null || tmux -L $L resize-pane -t w -x 50 -y 20
sleep 2; tmux -L $L capture-pane -p -t w > "$OUT/resize-100to50.txt"
tmux -L $L send-keys -t w q; sleep 1
tmux -L $L kill-server 2>/dev/null
ls "$OUT" | wc -l
