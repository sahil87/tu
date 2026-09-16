# tu(1) fish completion
# Install:
#   tu shell-init fish > ~/.config/fish/completions/tu.fish

# Non-data subcommands (first positional only)
complete -c tu -n '__fish_use_subcommand' -a 'help' -d 'show full help'
complete -c tu -n '__fish_use_subcommand' -a 'init-conf' -d 'scaffold ~/.config/tu/tu.conf'
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
complete -c tu -n '__fish_use_subcommand' -a 'kimi' -d 'Kimi'
complete -c tu -n '__fish_use_subcommand' -a 'ki' -d 'Kimi (alias)'
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
complete -c tu -n '__fish_use_subcommand' -a 'lb' -d 'leaderboard'
complete -c tu -n '__fish_use_subcommand' -a 'lbh' -d 'leaderboard history'

complete -c tu -n 'not __fish_use_subcommand' -a 'd w m daily weekly monthly h history dh wh mh lb lbh'

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
complete -c tu -l metric -r -a 'cost tokens' -d 'show cost or tokens'
complete -c tu -l top -r -d 'show only the top N leaderboard rows/columns'
complete -c tu -l sync -d 'sync metrics before fetch'
complete -c tu -l dry-run -d 'preview sync without writing'
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
complete -c tu -s t -d 'show tokens instead of cost'
complete -c tu -s v -d 'print version'
complete -c tu -s V -d 'print version'
complete -c tu -s h -d 'show help'
