export interface UsageTotals {
  totalCost: number;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
}

export interface UsageEntry extends UsageTotals {
  label: string; // date (2026-02-10) or month (2026-02)
}

export interface ToolConfig {
  name: string;
  command: string;
  needsFilter: boolean;
}
