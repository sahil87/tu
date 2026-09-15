#!/usr/bin/env bash
# Non-watch TTY captures (compact mode needs a real terminal; COLUMNS is ignored when piped). Real config (multi mode):
# own-user data commands write this machine's own day-files into the local metrics clone (never sync/push).
set -u
W="${WALK_OUT:-$(cd "$(dirname "$0")" && pwd)}"; OUT="$W/tty"; rm -rf "$OUT"; mkdir -p "$OUT"
TU=/home/linuxbrew/.linuxbrew/bin/tu; L=walktty
shot() { local name="$1" cols="$2" rows="$3"; shift 3
  tmux -L $L kill-server 2>/dev/null
  tmux -L $L new-session -d -s t -x "$cols" -y "$rows" "clear; $* ; echo __END__; sleep 60"
  for i in $(seq 1 40); do sleep 0.5; tmux -L $L capture-pane -p -t t -S -500 | grep -q __END__ && break; done
  tmux -L $L capture-pane -p -t t -S -500 | sed '/^$/N;/^\n$/D' | awk 'BEGIN{p=1} /__END__/{exit} {print}' > "$OUT/$name.txt"
  tmux -L $L kill-server 2>/dev/null; }
E="env NO_COLOR=1"
shot snap-50      50 40 $E $TU
shot snap-59      59 40 $E $TU
shot snap-60      60 40 $E $TU
shot snap-80      80 40 $E $TU
shot h-50         50 60 $E $TU h
shot h-59         59 60 $E $TU h
shot h-60         60 60 $E $TU h
shot h-80         80 60 $E $TU h
shot h-100        100 60 $E $TU h
shot h-120        120 60 $E $TU h
shot cc-h-50      50 60 $E $TU cc h
shot cc-h-80      80 60 $E $TU cc h
shot cc-h-100     100 60 $E $TU cc h
shot cc-h-120     120 60 $E $TU cc h
shot mh-50        50 40 $E $TU mh
shot lb-50        50 40 $E $TU m lb
shot lb-80        80 40 $E $TU m lb
shot lb-120       120 40 $E $TU m lb
shot lbh-80       80 40 $E $TU m lbh
shot lbh-140      140 40 $E $TU m lbh
shot bym-50       50 40 $E $TU m --by-machine
shot bym-120      120 40 $E $TU m --by-machine
shot snap-t-50    50 40 $E $TU -t
shot h-100-color  100 60 $TU h
shot snap-100-color 100 40 $TU
tmux -L $L kill-server 2>/dev/null
# colored escapes for the snapshot + lb at 100 (capture -e)
tmux -L $L new-session -d -s t -x 100 -y 40 "$TU; $TU m lb; echo __END__; sleep 60"
for i in $(seq 1 40); do sleep 0.5; tmux -L $L capture-pane -p -t t -S -500 | grep -q __END__ && break; done
tmux -L $L capture-pane -p -e -t t -S -500 > "$OUT/snap-lb-100-escapes.txt"
tmux -L $L kill-server 2>/dev/null
ls "$OUT" | wc -l
