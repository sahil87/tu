# tu(1) zsh completion
# Install:
#   echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc
#
# This snippet is intended for `eval`, not for autoload via $fpath. It defines
# the _tu function and registers it with `compdef`. compinit is loaded lazily
# if the user hasn't already done so, since `compdef` is unavailable until then.

_tu() {
  local -a non_data_subcommands sources periods display long_flags short_flags shells

  non_data_subcommands=(help init-conf init-metrics sync status update shell-init skill)
  sources=(cc codex co oc gemini gem copilot cop kimi ki all)
  periods=(d w m daily weekly monthly)
  display=(h history dh wh mh lb lbh)
  long_flags=(--json --csv --md --since --until --full --metric --top --sync --dry-run --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help)
  short_flags=(-f -w -i -u -s -j -t -v -V -h)
  shells=(bash zsh fish)

  local curcontext="$curcontext" state line
  typeset -A opt_args

  _arguments -C \
    '1: :->first' \
    '*: :->rest' \
    '--json[emit JSON]' \
    '-j[emit JSON]' \
    '--csv[emit CSV]' \
    '--md[emit Markdown]' \
    '--since[only include entries on/after date]:date:' \
    '-s[only include entries on/after date]:date:' \
    '--until[only include entries on/before date]:date:' \
    '--full[show full history (no 3-month cap)]' \
    '--metric[show cost or tokens]:metric:(cost tokens)' \
    '-t[show tokens instead of cost]' \
    '--top[show only the top N leaderboard rows/columns]:n:' \
    '--sync[sync metrics before fetch]' \
    '--dry-run[preview sync without writing]' \
    '--fresh[bypass cache]' \
    '-f[bypass cache]' \
    '--watch[persistent polling mode]' \
    '-w[persistent polling mode]' \
    '--interval[poll interval in seconds]:seconds:' \
    '-i[poll interval in seconds]:seconds:' \
    '--user[show usage for a specific user]:user:' \
    '-u[show usage for a specific user]:user:' \
    '--by-machine[per-machine cost breakdown]' \
    '--skip-brew-update[skip brew update tap refresh during tu update]' \
    '--no-color[disable ANSI colors]' \
    '--no-rain[disable matrix rain]' \
    '--version[print version]' \
    '-v[print version]' \
    '-V[print version]' \
    '--help[show help]' \
    '-h[show help]'

  case $state in
    first)
      _values 'command' \
        ${non_data_subcommands} \
        ${sources} \
        ${periods} \
        ${display}
      ;;
    rest)
      if [[ "${words[2]}" == "shell-init" ]]; then
        _values 'shell' ${shells}
      else
        _values 'token' \
          ${periods} \
          ${display}
      fi
      ;;
  esac
}

# Lazy-load compinit if the user hasn't already initialised the completion
# system — `compdef` is provided by compinit and is required to register _tu
# against the `tu` command at eval time.
(( $+functions[compdef] )) || { autoload -Uz compinit && compinit -i }
compdef _tu tu
