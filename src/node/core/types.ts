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
  binary: string;
  prefixArgs: string[];
  labelKey: string; // JSON key carrying the entry's ISO date label ("date" | "period")
  needsFilter: boolean;
}
