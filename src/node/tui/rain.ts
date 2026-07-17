// Matrix rain animation state machine
// Renders cursor-positioned falling characters in the rain zone

import { brightGreen, green, dimGreen } from "./colors.js";

// Character pools
const KATAKANA = "ｦｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ";
const DIGITS = "0123456789";
const LATIN = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
const CHAR_POOL = KATAKANA + DIGITS + LATIN;

const DENSITY = 0.3;
const DENSITY_REF_ROWS = 20; // zone height the current DENSITY was calibrated at
const MAX_DENSITY_SCALE = 3; // cap so very tall zones don't produce unbounded per-frame output
const MIN_SPEED = 0.3;
const MAX_SPEED = 1.0;
const MIN_LENGTH = 3;
const MAX_LENGTH = 8;
const MAX_RESPAWN_DELAY = 5;
const SHIMMER_RATE = 0.05;

function randFloat(min: number, max: number): number {
  return min + Math.random() * (max - min);
}

function randInt(min: number, max: number): number {
  return min + Math.floor(Math.random() * (max - min + 1));
}

function randChar(): string {
  return CHAR_POOL[Math.floor(Math.random() * CHAR_POOL.length)];
}

// Active drop count scales with rain-zone height: DENSITY is calibrated at
// DENSITY_REF_ROWS, and taller zones get proportionally more drops so coverage
// stays roughly constant. Clamped to 1x at/below the reference height
// (byte-identical to the prior width-only count) and capped at
// MAX_DENSITY_SCALE to bound per-frame output on very tall zones. With the
// shipped constants the count stays at or below `cols` (0.3 * 3 = 0.9); a
// column only hosts a second drop if DENSITY * MAX_DENSITY_SCALE is tuned
// above 1.
export function computeActiveDropCount(cols: number, rows: number): number {
  const heightScale = Math.min(MAX_DENSITY_SCALE, Math.max(1, rows / DENSITY_REF_ROWS));
  return Math.round(cols * DENSITY * heightScale);
}

interface Drop {
  col: number;
  row: number;       // fractional position (e.g., 5.6)
  speed: number;     // fractional rows per tick (0.3-1.0)
  length: number;    // trail length
  delay: number;     // ticks before starting
  chars: string[];   // pre-generated characters
}

export class RainState {
  private cols: number;
  private rows: number;
  private startCol: number;  // 0-based column offset for right-margin mode
  private drops: Drop[];
  private prevPositions: Set<string>; // "row,col" keys for previous frame

  constructor(cols: number, rows: number, startCol = 0) {
    this.cols = cols;
    this.rows = rows;
    this.startCol = startCol;
    this.drops = [];
    this.prevPositions = new Set();
    this.initDrops();
  }

  private initDrops(): void {
    this.drops = [];
    this.prevPositions = new Set();
    const activeCount = computeActiveDropCount(this.cols, this.rows);
    // Shuffle column indices, then assign drops round-robin over them: every
    // column is used once before any column would get a second drop (which
    // only happens if the constants are tuned so DENSITY * MAX_DENSITY_SCALE
    // exceeds 1 — shipped constants stay within one drop per column).
    const columns = Array.from({ length: this.cols }, (_, i) => i);
    for (let i = columns.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [columns[i], columns[j]] = [columns[j], columns[i]];
    }
    for (let i = 0; i < activeCount; i++) {
      this.drops.push(this.createDrop(columns[i % this.cols], true));
    }
  }

  private createDrop(col: number, scatter: boolean): Drop {
    const length = randInt(MIN_LENGTH, MAX_LENGTH);
    const chars = Array.from({ length }, () => randChar());
    return {
      col,
      row: scatter ? randFloat(-length, this.rows) : -length,
      speed: randFloat(MIN_SPEED, MAX_SPEED),
      length,
      delay: scatter ? 0 : randInt(0, MAX_RESPAWN_DELAY),
      chars,
    };
  }

  tick(): void {
    for (const drop of this.drops) {
      if (drop.delay > 0) {
        drop.delay--;
        continue;
      }
      drop.row += drop.speed;

      // Shimmer: randomly replace ~5% of trail chars
      for (let i = 0; i < drop.chars.length; i++) {
        if (Math.random() < SHIMMER_RATE) drop.chars[i] = randChar();
      }

      // Respawn when fully off-screen
      if (Math.floor(drop.row) - drop.length > this.rows) {
        const newDrop = this.createDrop(drop.col, false);
        drop.row = newDrop.row;
        drop.speed = newDrop.speed;
        drop.length = newDrop.length;
        drop.delay = newDrop.delay;
        drop.chars = newDrop.chars;
      }
    }
  }

  render(startRow: number): string {
    if (this.rows <= 0) return "";

    let output = "";
    const currentPositions = new Set<string>();

    for (const drop of this.drops) {
      if (drop.delay > 0) continue;

      const headRow = Math.floor(drop.row);

      for (let i = 0; i < drop.length; i++) {
        const row = headRow - i;
        if (row < 0 || row >= this.rows) continue;

        const screenRow = startRow + row;
        const screenCol = this.startCol + drop.col + 1; // 1-based for ANSI
        const char = drop.chars[i % drop.chars.length];

        const key = `${row},${drop.col}`;
        currentPositions.add(key);

        let colored: string;
        if (i === 0) {
          colored = brightGreen(char);
        } else if (i < drop.length - 2) {
          colored = green(char);
        } else {
          colored = dimGreen(char);
        }

        output += `\x1b[${screenRow};${screenCol}H${colored}`;
      }
    }

    // Clear old positions that are no longer occupied
    for (const key of this.prevPositions) {
      if (!currentPositions.has(key)) {
        const [r, c] = key.split(",").map(Number);
        const screenRow = startRow + r;
        const screenCol = this.startCol + c + 1; // 1-based for ANSI
        if (r >= 0 && r < this.rows) {
          output += `\x1b[${screenRow};${screenCol}H `;
        }
      }
    }

    this.prevPositions = currentPositions;

    return output;
  }

  resize(cols: number, rows: number, startCol = 0): void {
    if (cols === this.cols && rows === this.rows && startCol === this.startCol) return; // no-op
    this.cols = cols;
    this.rows = rows;
    this.startCol = startCol;
    this.initDrops();
  }
}
