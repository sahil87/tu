# Archive Index

- **260401-lomt-relicense-mit-migrate-sahil87** — Relicense from PolyForm-Internal-Use to MIT and migrate all org references from wvrdz to sahil87, aligning the codebase with the public repo and new Homebrew tap.
- **260401-jufw-remove-weaver-dev-derive-mode** — Remove WEAVER_DEV env var and derive mode from metrics_repo presence, eliminating project-specific config machinery and making the codebase fully generic.
- **260307-kpwv-add-by-machine-flag** — Add a --by-machine flag that shows per-machine cost distribution in multi-machine mode, using letter-coded columns with a legend mapping to machine names.
- **260307-9mfs-add-update-command** — Add a tu update self-update command that shells out to brew, so users can update from the CLI itself instead of remembering the manual Homebrew workflow.
- **260307-v2bu-filter-usage-by-user** — Fixed multi-mode to scope usage data to the current user only instead of aggregating all users in the metrics repo, and added a -u flag to optionally view another user's usage.
- **260306-x861-reorganize-src-node-namespace** — Reorganized flat src/ directory into src/node/ namespace with core/, tui/, and sync/ subdirectories, preparing for a future parallel Rust CLI implementation.
- **260306-mxla-redesign-watch-layout** — Redesigned watch mode layout: removed sparkline, moved session stats to top as 2x3 grid, simplified table below with rain background at 75% speed, and removed wide/medium breakpoint distinction.
- **260306-5hu8-fix-watch-mode-display-bugs** — Fixed two watch mode display bugs: tokens/min showing absurd values due to divide-by-near-zero on first render, and Total row displaying when only one tool has data.
