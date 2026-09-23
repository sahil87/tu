# Code Quality

## Principles

- Readability and maintainability over cleverness
- Follow existing project patterns unless there's compelling reason to deviate
- Prefer functions and plain objects over classes (functional style)
- Follow the constitution's Go Conventions: pure stages return values, errors are returned not printed, no result globals
- Keep minimum pathways — prefer fewer distinct code paths over optimizing away a read/write round-trip; a single well-exercised path catches more bugs than two paths that each run half as often

## Anti-Patterns

- God functions (>50 lines without clear reason)
- Duplicating existing utilities instead of reusing them
- Magic strings or numbers without named constants
- I/O, exec, or network in `init()` or package-level initializers (Constitution IV)
- Swallowing errors silently — always warn on stderr or return a meaningful fallback

## Test Strategy

test-alongside
