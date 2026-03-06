// Compositor: manages independent panel buffers with dirty-flag rendering
// Each panel has its own update cycle; the compositor tick only redraws dirty regions.
// Exception: RainLayer writes directly to stdout via its own setInterval,
// bypassing the compositor tick — this decouples rain from the rendering pipeline
// so it never freezes during API fetches.

import type { UsageEntry } from "../core/types.js";
import type { PanelSession } from "./panel.js";
import { buildPanel } from "./panel.js";
import { RainState } from "./rain.js";
import { dim, stripAnsi } from "./colors.js";

export interface PanelBuffer {
  render(): string[];
  readonly dirty: boolean;
}

export interface CompositorOptions {
  noRain?: boolean;
  getTermWidth: () => number;
  getTermRows: () => number;
}

const COMPOSITOR_TICK_MS = 16;
const RAIN_TICK_MS = 80;
const COUNTDOWN_TICK_MS = 1000;
const MIN_TABLE_WIDTH = 90;
const MIN_PANEL = 20;
const MERGE_GUTTER = 3;
const COMPACT_THRESHOLD = 60;

// --- TablePanel ---
class TablePanel implements PanelBuffer {
  dirty = false;

  private lines: string[] = [];

  update(lines: string[]): void {
    this.lines = lines;
    this.dirty = true;
  }

  render(): string[] {
    this.dirty = false;
    return this.lines;
  }
}

// --- SparkPanel ---
class SparkPanel implements PanelBuffer {
  dirty = false;

  private lines: string[] = [];

  update(session: PanelSession, history: UsageEntry[], panelWidth: number, todayCost: number): void {
    this.lines = buildPanel(session, history, panelWidth, todayCost);
    this.dirty = true;
  }

  render(): string[] {
    this.dirty = false;
    return this.lines;
  }

  getLines(): string[] {
    return this.lines;
  }
}

// --- StatusPanel ---
class StatusPanel implements PanelBuffer {
  dirty = false;

  private content = "";
  private countdown: number | null = null;
  private termWidth = 80;

  updateCountdown(value: number | null, termWidth: number): void {
    this.countdown = value;
    this.termWidth = termWidth;
    this.dirty = true;
  }

  render(): string[] {
    this.dirty = false;
    const parts: string[] = [];

    if (this.countdown === null) {
      parts.push(dim("Refreshing..."));
    } else {
      parts.push(dim(`Next refresh: ${this.countdown}s`));
    }

    parts.push(dim("\u21B5 refresh \u00B7 q quit"));

    // Progressive truncation
    let line = parts.join(dim(" \u00B7 "));
    while (stripAnsi(line).length > this.termWidth && parts.length > 1) {
      parts.pop();
      line = parts.join(dim(" \u00B7 "));
    }

    this.content = line;
    return [this.content];
  }

  getContent(): string {
    return this.content;
  }
}

// --- RainLayer ---
// RainLayer uses cursor-positioned writes as an overlay and does NOT trigger
// a full recomposite. Its tick/render cycle runs on its own setInterval,
// independent of the compositor tick.
class RainLayer {
  private rain: RainState | null = null;
  private startRow = 0;
  private enabled = false;

  setup(cols: number, availableRows: number, startRow: number, startCol = 0): void {
    if (availableRows <= 0 || cols <= 0) {
      this.enabled = false;
      this.rain = null;
      return;
    }
    this.enabled = true;
    this.startRow = startRow;
    if (!this.rain) {
      this.rain = new RainState(cols, availableRows, startCol);
    } else {
      this.rain.resize(cols, availableRows, startCol);
    }
  }

  disable(): void {
    this.enabled = false;
    this.rain = null;
  }

  tick(): void {
    if (!this.enabled || !this.rain) return;
    this.rain.tick();
  }

  renderDirect(): string {
    if (!this.enabled || !this.rain) return "";
    return this.rain.render(this.startRow);
  }

  isEnabled(): boolean {
    return this.enabled;
  }
}

// --- Compositor ---
export class Compositor {
  readonly table: TablePanel;
  readonly spark: SparkPanel;
  readonly status: StatusPanel;
  readonly rainLayer: RainLayer;

  private opts: CompositorOptions;
  private compositorTimer: ReturnType<typeof setInterval> | null = null;
  private rainTimer: ReturnType<typeof setInterval> | null = null;
  private countdownTimer: ReturnType<typeof setTimeout> | null = null;
  private countdownValue = 0;
  private lastShowPanel = false;

  // Cached data for resize re-rendering
  private lastTableLines: string[] = [];
  private lastSession: PanelSession | null = null;
  private lastHistory: UsageEntry[] = [];
  private lastTodayCost = 0;

  constructor(opts: CompositorOptions) {
    this.opts = opts;
    this.table = new TablePanel();
    this.spark = new SparkPanel();
    this.status = new StatusPanel();
    this.rainLayer = new RainLayer();
  }

  start(): void {
    // Compositor tick: check dirty flags and recomposite
    this.compositorTimer = setInterval(() => this.tick(), COMPOSITOR_TICK_MS);

    // Rain tick: independent ~80ms animation
    if (!this.opts.noRain) {
      this.rainTimer = setInterval(() => {
        this.rainLayer.tick();
        const rainOutput = this.rainLayer.renderDirect();
        if (rainOutput) process.stdout.write(rainOutput);
      }, RAIN_TICK_MS);
    }
  }

  stop(): void {
    if (this.compositorTimer !== null) {
      clearInterval(this.compositorTimer);
      this.compositorTimer = null;
    }
    if (this.rainTimer !== null) {
      clearInterval(this.rainTimer);
      this.rainTimer = null;
    }
    if (this.countdownTimer !== null) {
      clearTimeout(this.countdownTimer);
      this.countdownTimer = null;
    }
  }

  startCountdown(seconds: number, onExpire: () => void): void {
    this.countdownValue = seconds;
    this.status.updateCountdown(seconds, this.opts.getTermWidth());

    const tickDown = () => {
      this.countdownValue--;
      if (this.countdownValue <= 0) {
        onExpire();
      } else {
        this.status.updateCountdown(this.countdownValue, this.opts.getTermWidth());
        this.countdownTimer = setTimeout(tickDown, COUNTDOWN_TICK_MS);
      }
    };

    this.countdownTimer = setTimeout(tickDown, COUNTDOWN_TICK_MS);
  }

  cancelCountdown(): void {
    if (this.countdownTimer !== null) {
      clearTimeout(this.countdownTimer);
      this.countdownTimer = null;
    }
  }

  setRefreshing(): void {
    this.status.updateCountdown(null, this.opts.getTermWidth());
  }

  /** Called after API poll with new table data and panel session */
  updateAfterPoll(
    tableLines: string[],
    session: PanelSession,
    history: UsageEntry[],
    todayCost: number,
  ): void {
    // Cache data for resize re-rendering
    this.lastTableLines = tableLines;
    this.lastSession = session;
    this.lastHistory = history;
    this.lastTodayCost = todayCost;

    this.layoutAndUpdate(tableLines, session, history, todayCost);
  }

  /** Re-layout and flush using cached data (called on terminal resize) */
  rerender(): void {
    if (this.lastTableLines.length === 0) return;
    this.layoutAndUpdate(
      this.lastTableLines,
      this.lastSession!,
      this.lastHistory,
      this.lastTodayCost,
    );
    this.flush();
  }

  private layoutAndUpdate(
    tableLines: string[],
    session: PanelSession,
    history: UsageEntry[],
    todayCost: number,
  ): void {
    const tw = this.opts.getTermWidth();
    const compact = tw < COMPACT_THRESHOLD;
    const showPanel = tw >= (MIN_TABLE_WIDTH + MERGE_GUTTER + MIN_PANEL) && !compact;
    const panelWidth = showPanel ? Math.min(40, Math.max(MIN_PANEL, tw - MIN_TABLE_WIDTH - MERGE_GUTTER)) : 0;

    this.table.update(tableLines);
    this.lastShowPanel = showPanel;

    if (showPanel) {
      this.spark.update(session, history, panelWidth, todayCost);
    }

    // Compute merged height for rain zone calculation
    const sparkLines = showPanel ? this.spark.getLines() : [];
    const mergedHeight = Math.max(tableLines.length, sparkLines.length);

    // Setup rain zone
    const footerRow = 1;
    const termRows = this.opts.getTermRows();
    const availableRainRows = termRows - mergedHeight - footerRow;
    const wantRain = !this.opts.noRain && !compact;

    if (wantRain && availableRainRows > 0) {
      // Below-content rain (full width, rows below merged output)
      this.rainLayer.setup(tw, availableRainRows, mergedHeight + 1);
    } else if (wantRain) {
      // Right-margin rain: render in columns past the content width
      const maxContentWidth = this.computeMaxContentWidth(tableLines, sparkLines, showPanel);
      const MIN_RAIN_COLS = 10;
      const marginCols = tw - maxContentWidth;
      if (marginCols >= MIN_RAIN_COLS) {
        // Use all visible rows except footer for right-margin rain
        const rainRows = termRows - footerRow;
        this.rainLayer.setup(marginCols, rainRows, 1, maxContentWidth);
      } else {
        this.rainLayer.disable();
      }
    } else {
      this.rainLayer.disable();
    }
  }

  private computeMaxContentWidth(tableLines: string[], sparkLines: string[], showPanel: boolean): number {
    let maxWidth = 0;
    if (showPanel) {
      // When panel is shown, content width = tableWidth + gutter + panelWidth
      for (let i = 0; i < Math.max(tableLines.length, sparkLines.length); i++) {
        const tLen = stripAnsi(tableLines[i] ?? "").length;
        const pLen = stripAnsi(sparkLines[i] ?? "").length;
        const rowWidth = Math.max(tLen, MIN_TABLE_WIDTH) + MERGE_GUTTER + pLen;
        if (rowWidth > maxWidth) maxWidth = rowWidth;
      }
    } else {
      for (const line of tableLines) {
        const len = stripAnsi(line).length;
        if (len > maxWidth) maxWidth = len;
      }
    }
    return maxWidth;
  }

  /** Force a full composite and flush to screen */
  flush(): void {
    const tw = this.opts.getTermWidth();

    const tableLines = this.table.render();
    const sparkLines = this.lastShowPanel ? this.spark.render() : [];
    const merged = mergeSideBySide(tableLines, sparkLines, tw, this.lastShowPanel ? MIN_TABLE_WIDTH : tw);

    // Cursor home, write merged output
    process.stdout.write("\x1b[H");
    for (const line of merged) {
      process.stdout.write(line + "\n");
    }
    // Clear stale trailing lines
    process.stdout.write("\x1b[J");

    // Write footer
    const statusLines = this.status.render();
    if (statusLines.length > 0) {
      writeFooterLine(statusLines[0], this.opts.getTermRows());
    }
  }

  private tick(): void {
    // Full table/spark composites are driven via explicit flush() calls
    // (e.g., after a poll). The periodic tick is responsible only for
    // incremental status updates to avoid duplicate full-screen renders.
    if (this.status.dirty) {
      const statusLines = this.status.render();
      if (statusLines.length > 0) {
        writeFooterLine(statusLines[0], this.opts.getTermRows());
      }
    }
  }
}

// Shared utilities
function writeFooterLine(content: string, rows: number): void {
  process.stdout.write(`\x1b[${rows};1H\x1b[K${content}`);
}

export function mergeSideBySide(
  tableLines: string[],
  panelLines: string[],
  termWidth: number,
  tableWidth: number,
): string[] {
  if (panelLines.length === 0) return tableLines;

  const maxLen = Math.max(tableLines.length, panelLines.length);
  const result: string[] = [];

  for (let i = 0; i < maxLen; i++) {
    const tableLine = tableLines[i] ?? "";
    const panelLine = panelLines[i] ?? "";

    const visibleLen = stripAnsi(tableLine).length;
    const padding = Math.max(0, tableWidth - visibleLen);
    const gutter = " ".repeat(MERGE_GUTTER);

    result.push(tableLine + " ".repeat(padding) + gutter + panelLine);
  }

  return result;
}
