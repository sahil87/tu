# Specifications Index

> **Specs are pre-implementation artifacts** — what you *planned*. They capture conceptual design
> intent, high-level decisions, and the "why" behind features. Specs are human-curated,
> flat in structure, and deliberately size-controlled for quick reading.
>
> Contrast with [`docs/memory/index.md`](../memory/index.md): memory files are *post-implementation* —
> what actually happened. Memory files are the authoritative source of truth for system behavior,
> maintained by `/fab-archive` hydration.
>
> **Ownership**: Specs are written and maintained by humans. No automated tooling creates or
> enforces structure here — organize files however makes sense for your project.

| Spec | Description |
|------|-------------|
| [usage](usage.md) | The complete CLI contract: grammar, flags and their misuse policy, setup commands, toolkit contracts (`--version`, `help-dump`, `update`, `shell-init`, `skill`, config-home), exit codes, data flow, pinned JSON/CSV/Markdown shapes, table semantics, multi-machine sync, watch mode, and the **Drop at cutover** ledger of `[DECIDE]` markers for gate G0 |
| [layouts](layouts.md) | Verified mockups of every output layout: tables, token mode, machine columns, leaderboards and `--top`, CSV/Markdown/JSON shapes, watch mode and rain, status and help (verbatim), setup/sync/diagnostic messages, terminal-width behavior, color reference |
