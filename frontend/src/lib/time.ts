// Time / formatting helpers used by the live tail, query results, and
// detail panels. Keep everything in plain TypeScript so it tree-shakes well.

// relativeTime returns a human-readable "5s ago" / "3m ago" / "2h ago".
export function relativeTime(input: number | string | Date): string {
  const t = typeof input === "number" ? input : new Date(input).getTime();
  const diff = (Date.now() - t) / 1000;
  if (diff < 0) return "just now";
  if (diff < 1) return "just now";
  if (diff < 60) return `${Math.floor(diff)}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

// fmtTime renders a timestamp as a sortable short string.
export function fmtTime(input: number | string | Date): string {
  const d = new Date(input);
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

// fmtFull renders a timestamp in RFC3339 with millisecond precision.
export function fmtFull(input: number | string | Date): string {
  const d = new Date(input);
  return d.toISOString();
}

// duration renders ms as "230ms" / "1.4s" / "2m3s".
export function duration(ms: number): string {
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`;
  const mins = Math.floor(ms / 60_000);
  const secs = Math.floor((ms % 60_000) / 1000);
  return `${mins}m${secs}s`;
}

// ranges is a small set of canonical time-window presets in milliseconds.
export const TIME_RANGES = {
  "5m": 5 * 60 * 1000,
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
} as const;

export type TimeRangeKey = keyof typeof TIME_RANGES;

export function sinceMs(key: TimeRangeKey | ""): number {
  if (!key) return 0;
  return Date.now() - TIME_RANGES[key];
}
