// Static shell completion scripts for `tu shell-init <shell>`.
//
// These strings are emitted verbatim to stdout by the `shell-init` subcommand.
// Completion is done statically (no shell-out to `tu`) to keep tab-press
// latency near zero and to avoid coupling completion to the running binary.
// When the grammar changes, these scripts must be updated and the bundle
// rebuilt.

export const BASH_COMPLETION = `# tu(1) bash completion
# Install:
#   echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc

_tu_complete() {
  local cur prev words cword
  COMPREPLY=()
  cur="\${COMP_WORDS[COMP_CWORD]}"
  prev="\${COMP_WORDS[COMP_CWORD-1]}"

  local non_data_subcommands="help init-conf init-metrics sync status update shell-init skill"
  local sources="cc codex co oc gemini gem copilot cop all"
  local periods="d w m daily weekly monthly"
  local display="h history dh wh mh"
  local long_flags="--json --csv --md --since --until --full --sync --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help"
  local short_flags="-f -w -i -u -s -j -v -V -h"
  local shells="bash zsh fish"

  # Argument to --interval/--user/--since/--until takes a value; no completion
  case "\${prev}" in
    --interval|-i|--user|-u|--since|-s|--until)
      return 0
      ;;
    shell-init)
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

export const ZSH_COMPLETION = `# tu(1) zsh completion
# Install:
#   echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc
#
# This snippet is intended for \`eval\`, not for autoload via \$fpath. It defines
# the _tu function and registers it with \`compdef\`. compinit is loaded lazily
# if the user hasn't already done so, since \`compdef\` is unavailable until then.

_tu() {
  local -a non_data_subcommands sources periods display long_flags short_flags shells

  non_data_subcommands=(help init-conf init-metrics sync status update shell-init skill)
  sources=(cc codex co oc gemini gem copilot cop all)
  periods=(d w m daily weekly monthly)
  display=(h history dh wh mh)
  long_flags=(--json --csv --md --since --until --full --sync --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help)
  short_flags=(-f -w -i -u -s -j -v -V -h)
  shells=(bash zsh fish)

  local curcontext="$curcontext" state line
  typeset -A opt_args

  _arguments -C \\
    '1: :->first' \\
    '*: :->rest' \\
    '--json[emit JSON]' \\
    '-j[emit JSON]' \\
    '--csv[emit CSV]' \\
    '--md[emit Markdown]' \\
    '--since[only include entries on/after date]:date:' \\
    '-s[only include entries on/after date]:date:' \\
    '--until[only include entries on/before date]:date:' \\
    '--full[show full history (no 3-month cap)]' \\
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
    '--skip-brew-update[skip brew update tap refresh during tu update]' \\
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
      if [[ "\${words[2]}" == "shell-init" ]]; then
        _values 'shell' \${shells}
      else
        _values 'token' \\
          \${periods} \\
          \${display}
      fi
      ;;
  esac
}

# Lazy-load compinit if the user hasn't already initialised the completion
# system — \`compdef\` is provided by compinit and is required to register _tu
# against the \`tu\` command at eval time.
(( \$+functions[compdef] )) || { autoload -Uz compinit && compinit -i }
compdef _tu tu
`;

export const FISH_COMPLETION = `# tu(1) fish completion
# Install:
#   tu shell-init fish > ~/.config/fish/completions/tu.fish

# Non-data subcommands (first positional only)
complete -c tu -n '__fish_use_subcommand' -a 'help' -d 'show full help'
complete -c tu -n '__fish_use_subcommand' -a 'init-conf' -d 'scaffold ~/.tu.conf'
complete -c tu -n '__fish_use_subcommand' -a 'init-metrics' -d 'clone metrics repo'
complete -c tu -n '__fish_use_subcommand' -a 'sync' -d 'push/pull metrics'
complete -c tu -n '__fish_use_subcommand' -a 'status' -d 'show config and sync state'
complete -c tu -n '__fish_use_subcommand' -a 'update' -d 'update tu via Homebrew'
complete -c tu -n '__fish_use_subcommand' -a 'shell-init' -d 'emit shell init script'
complete -c tu -n '__fish_use_subcommand' -a 'skill' -d 'print agent usage bundle'

# Sources (first positional only)
complete -c tu -n '__fish_use_subcommand' -a 'cc' -d 'Claude Code'
complete -c tu -n '__fish_use_subcommand' -a 'codex' -d 'Codex'
complete -c tu -n '__fish_use_subcommand' -a 'co' -d 'Codex (alias)'
complete -c tu -n '__fish_use_subcommand' -a 'oc' -d 'OpenCode'
complete -c tu -n '__fish_use_subcommand' -a 'gemini' -d 'Gemini'
complete -c tu -n '__fish_use_subcommand' -a 'gem' -d 'Gemini (alias)'
complete -c tu -n '__fish_use_subcommand' -a 'copilot' -d 'Copilot'
complete -c tu -n '__fish_use_subcommand' -a 'cop' -d 'Copilot (alias)'
complete -c tu -n '__fish_use_subcommand' -a 'all' -d 'all tools (default)'

# Periods + display (any positional)
complete -c tu -n '__fish_use_subcommand' -a 'd' -d 'daily'
complete -c tu -n '__fish_use_subcommand' -a 'w' -d 'weekly'
complete -c tu -n '__fish_use_subcommand' -a 'm' -d 'monthly'
complete -c tu -n '__fish_use_subcommand' -a 'daily' -d 'daily'
complete -c tu -n '__fish_use_subcommand' -a 'weekly' -d 'weekly'
complete -c tu -n '__fish_use_subcommand' -a 'monthly' -d 'monthly'
complete -c tu -n '__fish_use_subcommand' -a 'h' -d 'history'
complete -c tu -n '__fish_use_subcommand' -a 'history' -d 'history'
complete -c tu -n '__fish_use_subcommand' -a 'dh' -d 'daily history'
complete -c tu -n '__fish_use_subcommand' -a 'wh' -d 'weekly history'
complete -c tu -n '__fish_use_subcommand' -a 'mh' -d 'monthly history'

complete -c tu -n 'not __fish_use_subcommand' -a 'd w m daily weekly monthly h history dh wh mh'

# Shells (only after 'shell-init')
complete -c tu -n '__fish_seen_subcommand_from shell-init' -a 'bash' -d 'emit bash completion'
complete -c tu -n '__fish_seen_subcommand_from shell-init' -a 'zsh' -d 'emit zsh completion'
complete -c tu -n '__fish_seen_subcommand_from shell-init' -a 'fish' -d 'emit fish completion'

# Long flags
complete -c tu -l json -d 'emit JSON'
complete -c tu -l csv -d 'emit CSV'
complete -c tu -l md -d 'emit Markdown'
complete -c tu -l since -r -d 'only include entries on/after date'
complete -c tu -l until -r -d 'only include entries on/before date'
complete -c tu -l full -d 'show full history (no 3-month cap)'
complete -c tu -l sync -d 'sync metrics before fetch'
complete -c tu -l fresh -d 'bypass cache'
complete -c tu -l watch -d 'persistent polling mode'
complete -c tu -l interval -r -d 'poll interval in seconds'
complete -c tu -l user -r -d 'show usage for a specific user'
complete -c tu -l by-machine -d 'per-machine cost breakdown'
complete -c tu -l skip-brew-update -d 'skip brew update tap refresh during tu update'
complete -c tu -l no-color -d 'disable ANSI colors'
complete -c tu -l no-rain -d 'disable matrix rain'
complete -c tu -l version -d 'print version'
complete -c tu -l help -d 'show help'

# Short flags
complete -c tu -s f -d 'bypass cache'
complete -c tu -s w -d 'persistent polling mode'
complete -c tu -s i -r -d 'poll interval in seconds'
complete -c tu -s u -r -d 'show usage for a specific user'
complete -c tu -s s -r -d 'only include entries on/after date'
complete -c tu -s j -d 'emit JSON'
complete -c tu -s v -d 'print version'
complete -c tu -s V -d 'print version'
complete -c tu -s h -d 'show help'
`;
