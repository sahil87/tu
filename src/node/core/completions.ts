// Static shell completion scripts for `tu completions <shell>`.
//
// These strings are emitted verbatim to stdout by the `completions` subcommand.
// Completion is done statically (no shell-out to `tu`) to keep tab-press
// latency near zero and to avoid coupling completion to the running binary.
// When the grammar changes, these scripts must be updated and the bundle
// rebuilt.

export const BASH_COMPLETION = `# tu(1) bash completion
# Install:
#   echo 'source <(tu completions bash)' >> ~/.bashrc

_tu_complete() {
  local cur prev words cword
  COMPREPLY=()
  cur="\${COMP_WORDS[COMP_CWORD]}"
  prev="\${COMP_WORDS[COMP_CWORD-1]}"

  local non_data_subcommands="help init-conf init-metrics sync status update completions"
  local sources="cc codex co oc all"
  local periods="d m daily monthly"
  local display="h history dh mh"
  local long_flags="--json --csv --md --sync --fresh --watch --interval --user --by-machine --no-color --no-rain --version --help"
  local short_flags="-f -w -i -u -v -V -h"
  local shells="bash zsh fish"

  # Argument to --interval/--user takes a value; no completion
  case "\${prev}" in
    --interval|-i|--user|-u)
      return 0
      ;;
    completions)
      COMPREPLY=( $(compgen -W "\${shells}" -- "\${cur}") )
      return 0
      ;;
  esac

  # Flag completion when current word starts with a dash
  if [[ "\${cur}" == --* ]]; then
    COMPREPLY=( $(compgen -W "\${long_flags}" -- "\${cur}") )
    return 0
  fi
  if [[ "\${cur}" == -* ]]; then
    COMPREPLY=( $(compgen -W "\${short_flags} \${long_flags}" -- "\${cur}") )
    return 0
  fi

  # First positional: subcommands + sources + periods + display tokens
  if [[ \${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "\${non_data_subcommands} \${sources} \${periods} \${display}" -- "\${cur}") )
    return 0
  fi

  # Subsequent positionals: periods + display tokens
  COMPREPLY=( $(compgen -W "\${periods} \${display} \${long_flags}" -- "\${cur}") )
  return 0
}

complete -F _tu_complete tu
`;

export const ZSH_COMPLETION = `#compdef tu
# tu(1) zsh completion
# Install:
#   tu completions zsh > "\${fpath[1]}/_tu"
#   autoload -Uz compinit && compinit

_tu() {
  local -a non_data_subcommands sources periods display long_flags short_flags shells

  non_data_subcommands=(help init-conf init-metrics sync status update completions)
  sources=(cc codex co oc all)
  periods=(d m daily monthly)
  display=(h history dh mh)
  long_flags=(--json --csv --md --sync --fresh --watch --interval --user --by-machine --no-color --no-rain --version --help)
  short_flags=(-f -w -i -u -v -V -h)
  shells=(bash zsh fish)

  local curcontext="$curcontext" state line
  typeset -A opt_args

  _arguments -C \\
    '1: :->first' \\
    '*: :->rest' \\
    '--json[emit JSON]' \\
    '--csv[emit CSV]' \\
    '--md[emit Markdown]' \\
    '--sync[sync metrics before fetch]' \\
    '--fresh[bypass cache]' \\
    '-f[bypass cache]' \\
    '--watch[persistent polling mode]' \\
    '-w[persistent polling mode]' \\
    '--interval[poll interval in seconds]:seconds:' \\
    '-i[poll interval in seconds]:seconds:' \\
    '--user[show usage for a specific user]:user:' \\
    '-u[show usage for a specific user]:user:' \\
    '--by-machine[per-machine cost breakdown]' \\
    '--no-color[disable ANSI colors]' \\
    '--no-rain[disable matrix rain]' \\
    '--version[print version]' \\
    '-v[print version]' \\
    '-V[print version]' \\
    '--help[show help]' \\
    '-h[show help]'

  case $state in
    first)
      _values 'command' \\
        \${non_data_subcommands} \\
        \${sources} \\
        \${periods} \\
        \${display}
      ;;
    rest)
      if [[ "\${words[2]}" == "completions" ]]; then
        _values 'shell' \${shells}
      else
        _values 'token' \\
          \${periods} \\
          \${display}
      fi
      ;;
  esac
}

_tu "$@"
`;

export const FISH_COMPLETION = `# tu(1) fish completion
# Install:
#   tu completions fish > ~/.config/fish/completions/tu.fish

# Non-data subcommands (first positional only)
complete -c tu -n '__fish_use_subcommand' -a 'help' -d 'show full help'
complete -c tu -n '__fish_use_subcommand' -a 'init-conf' -d 'scaffold ~/.tu.conf'
complete -c tu -n '__fish_use_subcommand' -a 'init-metrics' -d 'clone metrics repo'
complete -c tu -n '__fish_use_subcommand' -a 'sync' -d 'push/pull metrics'
complete -c tu -n '__fish_use_subcommand' -a 'status' -d 'show config and sync state'
complete -c tu -n '__fish_use_subcommand' -a 'update' -d 'update tu via Homebrew'
complete -c tu -n '__fish_use_subcommand' -a 'completions' -d 'emit shell completion script'

# Sources (first positional only)
complete -c tu -n '__fish_use_subcommand' -a 'cc' -d 'Claude Code'
complete -c tu -n '__fish_use_subcommand' -a 'codex' -d 'Codex'
complete -c tu -n '__fish_use_subcommand' -a 'co' -d 'Codex (alias)'
complete -c tu -n '__fish_use_subcommand' -a 'oc' -d 'OpenCode'
complete -c tu -n '__fish_use_subcommand' -a 'all' -d 'all tools (default)'

# Periods + display (any positional)
complete -c tu -n '__fish_use_subcommand' -a 'd' -d 'daily'
complete -c tu -n '__fish_use_subcommand' -a 'm' -d 'monthly'
complete -c tu -n '__fish_use_subcommand' -a 'daily' -d 'daily'
complete -c tu -n '__fish_use_subcommand' -a 'monthly' -d 'monthly'
complete -c tu -n '__fish_use_subcommand' -a 'h' -d 'history'
complete -c tu -n '__fish_use_subcommand' -a 'history' -d 'history'
complete -c tu -n '__fish_use_subcommand' -a 'dh' -d 'daily history'
complete -c tu -n '__fish_use_subcommand' -a 'mh' -d 'monthly history'

complete -c tu -n 'not __fish_use_subcommand' -a 'd m daily monthly h history dh mh'

# Shells (only after 'completions')
complete -c tu -n '__fish_seen_subcommand_from completions' -a 'bash' -d 'emit bash completion'
complete -c tu -n '__fish_seen_subcommand_from completions' -a 'zsh' -d 'emit zsh completion'
complete -c tu -n '__fish_seen_subcommand_from completions' -a 'fish' -d 'emit fish completion'

# Long flags
complete -c tu -l json -d 'emit JSON'
complete -c tu -l csv -d 'emit CSV'
complete -c tu -l md -d 'emit Markdown'
complete -c tu -l sync -d 'sync metrics before fetch'
complete -c tu -l fresh -d 'bypass cache'
complete -c tu -l watch -d 'persistent polling mode'
complete -c tu -l interval -r -d 'poll interval in seconds'
complete -c tu -l user -r -d 'show usage for a specific user'
complete -c tu -l by-machine -d 'per-machine cost breakdown'
complete -c tu -l no-color -d 'disable ANSI colors'
complete -c tu -l no-rain -d 'disable matrix rain'
complete -c tu -l version -d 'print version'
complete -c tu -l help -d 'show help'

# Short flags
complete -c tu -s f -d 'bypass cache'
complete -c tu -s w -d 'persistent polling mode'
complete -c tu -s i -r -d 'poll interval in seconds'
complete -c tu -s u -r -d 'show usage for a specific user'
complete -c tu -s v -d 'print version'
complete -c tu -s V -d 'print version'
complete -c tu -s h -d 'show help'
`;
