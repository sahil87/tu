# tu(1) bash completion
# Install:
#   echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc

_tu_complete() {
  local cur prev words cword
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  local non_data_subcommands="help init-conf init-metrics sync status update shell-init skill"
  local sources="cc codex co oc gemini gem copilot cop kimi ki all"
  local periods="d w m daily weekly monthly"
  local display="h history dh wh mh lb lbh"
  local long_flags="--json --csv --md --since --until --full --metric --top --sync --dry-run --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help"
  local short_flags="-f -w -i -u -s -j -t -v -V -h"
  local shells="bash zsh fish"

  # Argument to --interval/--user/--since/--until/--top takes a value; no completion
  case "${prev}" in
    --interval|-i|--user|-u|--since|-s|--until|--top)
      return 0
      ;;
    --metric)
      COMPREPLY=( $(compgen -W "cost tokens" -- "${cur}") )
      return 0
      ;;
    shell-init)
      COMPREPLY=( $(compgen -W "${shells}" -- "${cur}") )
      return 0
      ;;
  esac

  # Flag completion when current word starts with a dash
  if [[ "${cur}" == --* ]]; then
    COMPREPLY=( $(compgen -W "${long_flags}" -- "${cur}") )
    return 0
  fi
  if [[ "${cur}" == -* ]]; then
    COMPREPLY=( $(compgen -W "${short_flags} ${long_flags}" -- "${cur}") )
    return 0
  fi

  # First positional: subcommands + sources + periods + display tokens
  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${non_data_subcommands} ${sources} ${periods} ${display}" -- "${cur}") )
    return 0
  fi

  # Subsequent positionals: periods + display tokens
  COMPREPLY=( $(compgen -W "${periods} ${display} ${long_flags}" -- "${cur}") )
  return 0
}

complete -F _tu_complete tu
