import { describe, it, mock } from "node:test";
import assert from "node:assert/strict";

import { runShellInit } from "../cli.js";
import { BASH_COMPLETION, ZSH_COMPLETION, FISH_COMPLETION } from "../completions.js";

// ---------------------------------------------------------------------------
// Helpers: capture stdout, stderr, and process.exit for `runShellInit`.
// runShellInit writes the script to process.stdout.write (not console.log)
// for bash/zsh/fish; usage errors (no arg, unknown shell) print to
// console.error and call process.exit(2), leaving stdout empty — per the
// toolkit shell-init standard (stdout may be eval'd by shells).
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
  "--since",
  "--until",
  "--full",
  "--metric",
  "--total",
  "--sync",
  "--dry-run",
  "--fresh",
  "--watch",
  "--interval",
  "--user",
  "--by-machine",
  "--skip-brew-update",
  "--no-color",
  "--no-rain",
  "--version",
  "--help",
];

// Short flags that must appear literally in every completion script.
const SHORT_FLAGS = ["-f", "-w", "-i", "-u", "-s", "-j", "-t", "-v", "-V", "-h"];

const NON_DATA_SUBCOMMANDS = [
  "help",
  "init-conf",
  "init-metrics",
  "sync",
  "status",
  "update",
  "shell-init",
  "skill",
];

// ---------------------------------------------------------------------------
// runShellInit: per-shell dispatch
// ---------------------------------------------------------------------------

describe("runShellInit: bash", () => {
  it("writes the bash script to stdout and does not exit with failure", (t) => {
    t.after(restore);
    const cap = captureIo();
    runShellInit("bash");
    const out = cap.stdout.join("");
    assert.ok(out.includes("complete -F _tu_complete tu"), "bash script should register completion via `complete`");
    assert.notEqual(cap.exitCode, 1);
  });
});

describe("runShellInit: zsh", () => {
  it("emits an eval-able snippet that registers _tu via compdef and does not auto-invoke", (t) => {
    t.after(restore);
    const cap = captureIo();
    runShellInit("zsh");
    const out = cap.stdout.join("");
    assert.ok(out.includes("compdef _tu tu"), "zsh script must register _tu against tu via compdef");
    assert.ok(!/^#compdef\s+tu/m.test(out), "zsh script must NOT use #compdef autoload magic — it would silently no-op under eval");
    assert.ok(!/\n_tu\s+"\$@"/m.test(out), "zsh script must NOT auto-invoke _tu at load time — that would run completion code at shell startup");
    assert.notEqual(cap.exitCode, 1);
  });
});

describe("runShellInit: fish", () => {
  it("writes the fish script to stdout with complete -c tu directives", (t) => {
    t.after(restore);
    const cap = captureIo();
    runShellInit("fish");
    const out = cap.stdout.join("");
    assert.ok(out.includes("complete -c tu"), "fish script should use `complete -c tu`");
    assert.notEqual(cap.exitCode, 1);
  });
});

describe("runShellInit: no argument", () => {
  // Toolkit shell-init standard: missing shell arg is a usage error — usage on
  // stderr, exit 2, stdout EMPTY (stdout may be eval'd by shells).
  it("prints usage + install examples to stderr, exits 2, and emits nothing on stdout", (t) => {
    t.after(restore);
    const cap = captureIo();
    runShellInit(undefined);
    const err = cap.errors.join("\n");
    assert.ok(err.includes("Usage: tu shell-init <bash|zsh|fish>"), "usage heading on stderr");
    assert.ok(err.includes("bash"), "mentions bash install");
    assert.ok(err.includes("zsh"), "mentions zsh install");
    assert.ok(err.includes("fish"), "mentions fish install");
    assert.equal(cap.exitCode, 2);
    assert.deepEqual(cap.stdout, [], "stdout must be empty");
    assert.deepEqual(cap.logs, [], "console.log must not be used");
  });
});

describe("runShellInit: unknown shell", () => {
  it("emits stderr message and exits 2", (t) => {
    t.after(restore);
    const cap = captureIo();
    runShellInit("powershell");
    assert.equal(cap.exitCode, 2);
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

describe("completion script coverage — short flags", () => {
  for (const flag of SHORT_FLAGS) {
    const letter = flag.replace(/^-/, "");
    it(`bash script contains literal ${flag}`, () => {
      assert.ok(BASH_COMPLETION.includes(flag), `bash missing ${flag}`);
    });
    it(`zsh script contains literal ${flag}`, () => {
      assert.ok(ZSH_COMPLETION.includes(flag), `zsh missing ${flag}`);
    });
    it(`fish script contains ${flag} (as -s ${letter})`, () => {
      // fish declares short flags via `-s <letter>`, e.g. `complete -c tu -s j`.
      assert.ok(FISH_COMPLETION.includes(`-s ${letter}`), `fish missing -s ${letter}`);
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

describe("completion script coverage — sources, periods, display, shell-init args", () => {
  const TOKENS = [
    // Sources
    "cc", "codex", "co", "oc", "gemini", "gem", "copilot", "cop", "kimi", "ki", "all",
    // Periods
    "d", "w", "m", "daily", "weekly", "monthly",
    // Display
    "h", "history", "dh", "wh", "mh",
    // `shell-init` args
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
