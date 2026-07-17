// Compositor: manages independent panel buffers with dirty-flag rendering
// Each panel has its own update cycle; the compositor tick only redraws dirty regions.
// Exception: RainLayer writes directly to stdout via its own setInterval,
// bypassing the compositor tick — this decouples rain from the rendering pipeline
// so it never freezes during API fetches.

import type { PanelSession } from "./panel.js";
import { buildStatsGrid } from "./panel.js";
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

const RAIN_TICK_MS = 107;
const COUNTDOWN_TICK_MS = 1000;
export const COMPACT_THRESHOLD = 60;

// Right-margin rain: minimum usable width, and gutter columns kept between the
// widest content line and the first rain column so glyphs don't touch the table.
const MIN_RAIN_COLS = 10;
const RAIN_GUTTER = 2;

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

// --- StatsPanel ---
class StatsPanel implements PanelBuffer {
  dirty = false;

  private lines: string[] = [];

  update(session: PanelSession, todayCost: number): void {
    this.lines = buildStatsGrid(session, todayCost);
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
  readonly stats: StatsPanel;
  readonly status: StatusPanel;
  readonly rainLayer: RainLayer;

  private opts: CompositorOptions;
  private rainTimer: ReturnType<typeof setInterval> | null = null;
  private countdownTimer: ReturnType<typeof setTimeout> | null = null;
  private countdownValue = 0;

  // Cached data for resize re-rendering
  private lastTableLines: string[] = [];
  private lastSession: PanelSession | null = null;
  private lastTodayCost = 0;

  constructor(opts: CompositorOptions) {
    this.opts = opts;
    this.table = new TablePanel();
    this.stats = new StatsPanel();
    this.status = new StatusPanel();
    this.rainLayer = new RainLayer();
  }

  start(): void {
    // Rain tick: independent ~107ms animation (~75% as fast as the original 80ms rate)
    if (!this.opts.noRain) {
      this.rainTimer = setInterval(() => {
        this.rainLayer.tick();
        const rainOutput = this.rainLayer.renderDirect();
        if (rainOutput) process.stdout.write(rainOutput);
      }, RAIN_TICK_MS);
    }
  }

  stop(): void {
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
    this.renderStatus();

    const tickDown = () => {
      this.countdownValue--;
      if (this.countdownValue <= 0) {
        onExpire();
      } else {
        this.status.updateCountdown(this.countdownValue, this.opts.getTermWidth());
        this.renderStatus();
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
    this.renderStatus();
  }

  /** Called after API poll with new table data and panel session */
  updateAfterPoll(
    tableLines: string[],
    session: PanelSession,
    todayCost: number,
  ): void {
    // Cache data for resize re-rendering
    this.lastTableLines = tableLines;
    this.lastSession = session;
    this.lastTodayCost = todayCost;

    this.layoutAndUpdate(tableLines, session, todayCost);
  }

  /** Re-layout and flush using cached data (called on terminal resize) */
  rerender(): void {
    if (this.lastTableLines.length === 0) return;
    this.layoutAndUpdate(
      this.lastTableLines,
      this.lastSession!,
      this.lastTodayCost,
    );
    this.flush();
  }

  private layoutAndUpdate(
    tableLines: string[],
    session: PanelSession,
    todayCost: number,
  ): void {
    const tw = this.opts.getTermWidth();
    const compact = tw < COMPACT_THRESHOLD;

    this.table.update(tableLines);

    if (!compact) {
      this.stats.update(session, todayCost);
    }

    // Compute content height for rain zone calculation
    const statsLines = compact ? [] : this.stats.getLines();
    const contentHeight = statsLines.length + tableLines.length;
    const maxContentWidth = this.computeMaxContentWidth([...statsLines, ...tableLines]);

    this.setupRainZone(contentHeight, maxContentWidth);
  }

  /**
   * Lay out the rain zone against loading-skeleton geometry so rain animates
   * before the first poll completes. Derives content height and max content
   * width from the skeleton lines and reuses the shared rain-zone setup.
   */
  layoutForSkeleton(lines: string[]): void {
    const contentHeight = lines.length;
    const maxContentWidth = this.computeMaxContentWidth(lines);
    this.setupRainZone(contentHeight, maxContentWidth);
  }

  /** Shared rain-zone computation used by both poll layout and skeleton layout. */
  private setupRainZone(contentHeight: number, maxContentWidth: number): void {
    const tw = this.opts.getTermWidth();
    const compact = tw < COMPACT_THRESHOLD;

    const footerRow = 1;
    const termRows = this.opts.getTermRows();
    const availableRainRows = termRows - contentHeight - footerRow;
    const wantRain = !this.opts.noRain && !compact;

    if (wantRain && availableRainRows > 0) {
      // Below-content rain (full width, rows below content)
      this.rainLayer.setup(tw, availableRainRows, contentHeight + 1);
    } else if (wantRain) {
      // Right-margin rain: render in columns past the content width, keeping a
      // gutter so glyphs don't visually collide with the table edge.
      const marginCols = tw - maxContentWidth - RAIN_GUTTER;
      if (marginCols >= MIN_RAIN_COLS) {
        const rainRows = termRows - footerRow;
        this.rainLayer.setup(marginCols, rainRows, 1, maxContentWidth + RAIN_GUTTER);
      } else {
        this.rainLayer.disable();
      }
    } else {
      this.rainLayer.disable();
    }
  }

  private computeMaxContentWidth(lines: string[]): number {
    let maxWidth = 0;
    for (const line of lines) {
      const len = stripAnsi(line).length;
      if (len > maxWidth) maxWidth = len;
    }
    return maxWidth;
  }

  /** Force a full composite and flush to screen */
  flush(): void {
    const tw = this.opts.getTermWidth();
    const compact = tw < COMPACT_THRESHOLD;

    const statsLines = compact ? [] : this.stats.render();
    const tableLines = this.table.render();
    const allLines = [...statsLines, ...tableLines];

    // Cursor home, write output
    process.stdout.write("\x1b[H");
    for (const line of allLines) {
      process.stdout.write(line + "\x1b[K\n");
    }
    // Clear stale trailing lines
    process.stdout.write("\x1b[J");

    // Write footer
    const statusLines = this.status.render();
    if (statusLines.length > 0) {
      writeFooterLine(statusLines[0], this.opts.getTermRows());
    }

    // Restore the rain zone: flush's \x1b[J/\x1b[K clears erased every drawn
    // rain cell. RainState.render() rewrites every occupied cell each frame, so
    // re-emitting it here fully restores the rain without waiting for the next
    // rain tick (which would otherwise leave a per-poll blink).
    const rainOut = this.rainLayer.renderDirect();
    if (rainOut) process.stdout.write(rainOut);
  }

  /**
   * Render the footer status line immediately. Push-driven — called after every
   * countdown/refresh state change (replaces the former periodic 16ms tick).
   */
  private renderStatus(): void {
    const statusLines = this.status.render();
    if (statusLines.length > 0) {
      writeFooterLine(statusLines[0], this.opts.getTermRows());
    }
  }
}

// Shared utilities
function writeFooterLine(content: string, rows: number): void {
  process.stdout.write(`\x1b[${rows};1H\x1b[K${content}`);
}
