# Archive Index

- **260307-v2bu-filter-usage-by-user** — Fixed multi-mode to scope usage data to the current user only instead of aggregating all users in the metrics repo, and added a -u flag to optionally view another user's usage.
- **260306-x861-reorganize-src-node-namespace** — Reorganized flat src/ directory into src/node/ namespace with core/, tui/, and sync/ subdirectories, preparing for a future parallel Rust CLI implementation.
- **260306-mxla-redesign-watch-layout** — Redesigned watch mode layout: removed sparkline, moved session stats to top as 2x3 grid, simplified table below with rain background at 75% speed, and removed wide/medium breakpoint distinction.
- **260306-5hu8-fix-watch-mode-display-bugs** — Fixed two watch mode display bugs: tokens/min showing absurd values due to divide-by-near-zero on first render, and Total row displaying when only one tool has data.
