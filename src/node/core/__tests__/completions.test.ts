import { describe, it, mock } from "node:test";
import assert from "node:assert/strict";

import { runCompletions } from "../cli.js";
import { BASH_COMPLETION, ZSH_COMPLETION, FISH_COMPLETION } from "../completions.js";

// ---------------------------------------------------------------------------
// Helpers: capture stdout, stderr, and process.exit for `runCompletions`.
// runCompletions writes the script to process.stdout.write (not console.log)
// for bash/zsh/fish, prints usage via console.log for the no-arg case, and
// prints the error to console.error + calls process.exit(1) for unknown shells.
// ---------------------------------------------------------------------------

interface Capture {
  stdout: string[];
  logs: string[];
  errors: string[];
  exitCode: number | null;
}

function captureIo(): Capture {
  const cap: Capture = { stdout: [], logs: [], errors: [], exitCode: null };
  mock.method(process.stdout, "write", ((chunk: string | Uint8Array) => {
    cap.stdout.push(typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf-8"));
    return true;
  }) as never);
  mock.method(console, "log", ((...args: unknown[]) => {
    cap.logs.push(args.map(String).join(" "));
  }) as never);
  mock.method(console, "error", ((...args: unknown[]) => {
    cap.errors.push(args.map(String).join(" "));
  }) as never);
  mock.method(process, "exit", ((code: number) => {
    cap.exitCode = code;
  }) as never);
  return cap;
}

function restore() {
  mock.restoreAll();
}

// Flag taxonomy used by spec requirement "Completion Script Coverage".
const LONG_FLAGS = [
  "--json",
  "--csv",
  "--md",
  "--sync",
  "--fresh",
  "--watch",
  "--interval",
  "--user",
  "--by-machine",
  "--no-color",
  "--no-rain",
  "--version",
  "--help",
];

const NON_DATA_SUBCOMMANDS = [
  "help",
  "init-conf",
  "init-metrics",
  "sync",
  "status",
  "update",
  "completions",
];

// ---------------------------------------------------------------------------
// runCompletions: per-shell dispatch
// ---------------------------------------------------------------------------

describe("runCompletions: bash", () => {
  it("writes the bash script to stdout and does not exit with failure", (t) => {
    t.after(restore);
    const cap = captureIo();
    runCompletions("bash");
    const out = cap.stdout.join("");
    assert.ok(out.includes("complete -F _tu_complete tu"), "bash script should register completion via `complete`");
    assert.notEqual(cap.exitCode, 1);
  });
});

describe("runCompletions: zsh", () => {
  it("writes the zsh script to stdout with #compdef tu", (t) => {
    t.after(restore);
    const cap = captureIo();
    runCompletions("zsh");
    const out = cap.stdout.join("");
    assert.ok(out.includes("#compdef tu"), "zsh script should begin with #compdef tu");
    assert.notEqual(cap.exitCode, 1);
  });
});

describe("runCompletions: fish", () => {
  it("writes the fish script to stdout with complete -c tu directives", (t) => {
    t.after(restore);
    const cap = captureIo();
    runCompletions("fish");
    const out = cap.stdout.join("");
    assert.ok(out.includes("complete -c tu"), "fish script should use `complete -c tu`");
    assert.notEqual(cap.exitCode, 1);
  });
});

describe("runCompletions: no argument", () => {
  it("prints usage + install examples and does not exit with failure", (t) => {
    t.after(restore);
    const cap = captureIo();
    runCompletions(undefined);
    const out = cap.logs.join("\n");
    assert.ok(out.includes("Usage: tu completions <bash|zsh|fish>"), "usage heading");
    assert.ok(out.includes("bash"), "mentions bash install");
    assert.ok(out.includes("zsh"), "mentions zsh install");
    assert.ok(out.includes("fish"), "mentions fish install");
    assert.notEqual(cap.exitCode, 1);
  });
});

describe("runCompletions: unknown shell", () => {
  it("emits stderr message and exits 1", (t) => {
    t.after(restore);
    const cap = captureIo();
    runCompletions("powershell");
    assert.equal(cap.exitCode, 1);
    assert.ok(
      cap.errors.some((e) => e.includes("Unknown shell: powershell. Supported: bash, zsh, fish")),
      `expected stderr message; got: ${cap.errors.join("; ")}`,
    );
  });
});

// ---------------------------------------------------------------------------
// Completion script coverage: each script contains every token in the
// spec's Completion Script Coverage Requirement.
// ---------------------------------------------------------------------------

describe("completion script coverage — long flags", () => {
  for (const flag of LONG_FLAGS) {
    it(`bash script contains literal ${flag}`, () => {
      assert.ok(BASH_COMPLETION.includes(flag), `bash missing ${flag}`);
    });
    it(`zsh script contains literal ${flag}`, () => {
      assert.ok(ZSH_COMPLETION.includes(flag), `zsh missing ${flag}`);
    });
    it(`fish script contains literal ${flag}`, () => {
      // fish uses -l stripped-of-leading-dashes, e.g. `complete -c tu -l json`.
      // We check both the stripped form and the full form to be robust.
      const stripped = flag.replace(/^--/, "");
      assert.ok(
        FISH_COMPLETION.includes(flag) || FISH_COMPLETION.includes(` ${stripped} `) || FISH_COMPLETION.includes(`-l ${stripped}`),
        `fish missing ${flag} (or its -l ${stripped} form)`,
      );
    });
  }
});

describe("completion script coverage — non-data subcommands", () => {
  for (const cmd of NON_DATA_SUBCOMMANDS) {
    it(`bash script contains literal ${cmd}`, () => {
      assert.ok(BASH_COMPLETION.includes(cmd), `bash missing ${cmd}`);
    });
    it(`zsh script contains literal ${cmd}`, () => {
      assert.ok(ZSH_COMPLETION.includes(cmd), `zsh missing ${cmd}`);
    });
    it(`fish script contains literal ${cmd}`, () => {
      assert.ok(FISH_COMPLETION.includes(cmd), `fish missing ${cmd}`);
    });
  }
});

describe("completion script coverage — sources, periods, display, completion args", () => {
  const TOKENS = [
    // Sources
    "cc", "codex", "co", "oc", "all",
    // Periods
    "d", "m", "daily", "monthly",
    // Display
    "h", "history", "dh", "mh",
    // `completions` args
    "bash", "zsh", "fish",
  ];

  for (const token of TOKENS) {
    it(`bash script contains ${token}`, () => {
      assert.ok(BASH_COMPLETION.includes(token), `bash missing ${token}`);
    });
    it(`zsh script contains ${token}`, () => {
      assert.ok(ZSH_COMPLETION.includes(token), `zsh missing ${token}`);
    });
    it(`fish script contains ${token}`, () => {
      assert.ok(FISH_COMPLETION.includes(token), `fish missing ${token}`);
    });
  }
});
