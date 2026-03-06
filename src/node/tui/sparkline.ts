// Braille-dot sparkline chart renderer
// Uses Unicode braille patterns (U+2800-U+28FF) for smooth resolution

import { green, dim } from "./colors.js";

// Braille character: 2 columns × 4 rows per character cell
// Dot positions (bit index):
//   col0: row0=0, row1=1, row2=2, row3=6
//   col1: row0=3, row1=4, row2=5, row3=7
const DOT_BITS = [
  [0x01, 0x02, 0x04, 0x40], // left column: rows 0-3
  [0x08, 0x10, 0x20, 0x80], // right column: rows 0-3
];

const BRAILLE_BASE = 0x2800;

export function renderSparkline(
  data: Array<{ label: string; cost: number }>,
  width: number,
): string[] {
  if (data.length < 2) return [];

  // Pre-scan costs to compute label width before chart width
  const allCosts = data.map((d) => d.cost);
  const preMaxCost = Math.max(...allCosts);
  const preMinCost = Math.min(...allCosts);
  const preMaxLabel = `$${Math.round(preMaxCost)}`;
  const preMinLabel = `$${Math.round(preMinCost)}`;
  const labelReserve = Math.max(preMaxLabel.length, preMinLabel.length) + 2; // label + " ┤"

  const chartWidth = Math.max(4, Math.min(width - labelReserve, 40));
  const maxPoints = chartWidth * 2; // 2 data points per braille char
  const points = data.slice(-maxPoints);

  const costs = points.map((d) => d.cost);
  const maxCost = Math.max(...costs);
  const minCost = Math.min(...costs);

  const chartHeight = 3; // braille rows (each char = 4 dot rows)
  const totalDotRows = chartHeight * 4;

  const lines: string[] = [];

  // Title: show date range
  const firstLabel = points[0].label;
  const lastLabel = points[points.length - 1].label;
  lines.push(dim(` ${firstLabel} \u2192 ${lastLabel}`));

  // Build braille grid
  const charCols = Math.ceil(points.length / 2);
  const grid: number[][] = Array.from({ length: chartHeight }, () =>
    Array(charCols).fill(0),
  );

  const range = maxCost - minCost;

  for (let i = 0; i < points.length; i++) {
    const cost = costs[i];
    // Normalize to 0..1 (inverted: 0=top, 1=bottom)
    const normalized = range > 0 ? (cost - minCost) / range : 0.5;
    // Map to dot row (0=bottom, totalDotRows-1=top)
    const dotRow = Math.round(normalized * (totalDotRows - 1));
    // Invert: row 0 = top of chart
    const invertedRow = totalDotRows - 1 - dotRow;

    const charCol = Math.floor(i / 2);
    const subCol = i % 2; // 0=left dot column, 1=right dot column
    const charRow = Math.floor(invertedRow / 4);
    const subRow = invertedRow % 4;

    if (charRow >= 0 && charRow < chartHeight && charCol < charCols) {
      grid[charRow][charCol] |= DOT_BITS[subCol][subRow];
    }
  }

  // Render grid rows with y-axis labels
  const maxLabel = `$${Math.round(maxCost)}`;
  const minLabel = `$${Math.round(minCost)}`;
  const labelWidth = Math.max(maxLabel.length, minLabel.length);

  for (let row = 0; row < chartHeight; row++) {
    let label = "";
    if (row === 0) label = maxLabel.padStart(labelWidth);
    else if (row === chartHeight - 1) label = minLabel.padStart(labelWidth);
    else label = " ".repeat(labelWidth);

    const braille = grid[row]
      .map((bits) => String.fromCharCode(BRAILLE_BASE + bits))
      .join("");

    lines.push(dim(label + " \u2524") + green(braille));
  }

  return lines;
}
