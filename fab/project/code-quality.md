# Code Quality

## Principles

- Readability and maintainability over cleverness
- Follow existing project patterns unless there's compelling reason to deviate
- Prefer functions and plain objects over classes (functional style)
- Use `type` imports for type-only values
- Use `node:` prefixed imports for built-in modules
- Keep minimum pathways — prefer fewer distinct code paths over optimizing away a read/write round-trip; a single well-exercised path catches more bugs than two paths that each run half as often

## Anti-Patterns

- God functions (>50 lines without clear reason)
- Duplicating existing utilities instead of reusing them
- Magic strings or numbers without named constants
- Dynamic `import()` for core paths (hurts startup latency)
- Swallowing errors silently — always warn on stderr or return a meaningful fallback

## Test Strategy

test-alongside
