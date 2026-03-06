// ANSI color helpers with NO_COLOR support
// See: https://no-color.org/

let _noColor = false;

export function setNoColor(flag: boolean): void {
  _noColor = flag;
}

function isColorDisabled(): boolean {
  return _noColor || !!process.env.NO_COLOR;
}

function wrap(code: string, reset: string, s: string): string {
  if (isColorDisabled()) return s;
  return `${code}${s}${reset}`;
}

const RESET = "\x1b[0m";

export function bold(s: string): string {
  return wrap("\x1b[1m", RESET, s);
}

export function dim(s: string): string {
  return wrap("\x1b[2m", RESET, s);
}

export function green(s: string): string {
  return wrap("\x1b[32m", RESET, s);
}

export function red(s: string): string {
  return wrap("\x1b[31m", RESET, s);
}

export function cyan(s: string): string {
  return wrap("\x1b[36m", RESET, s);
}

export function yellow(s: string): string {
  return wrap("\x1b[33m", RESET, s);
}

export function boldWhite(s: string): string {
  return wrap("\x1b[1;37m", RESET, s);
}

export function boldCyan(s: string): string {
  return wrap("\x1b[1;36m", RESET, s);
}

export function brightGreen(s: string): string {
  return wrap("\x1b[92m", RESET, s);
}

export function dimGreen(s: string): string {
  return wrap("\x1b[2;32m", RESET, s);
}

// Strip ANSI codes for length measurement
export function stripAnsi(s: string): string {
  return s.replace(/\x1b\[[0-9;]*m/g, "");
}
