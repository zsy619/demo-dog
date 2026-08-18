// A tiny numeric-stat card with a delta indicator. Used by Overview.
//
// The numeric value animates with CountUp so when a new value arrives the user
// sees the change visually instead of the digit just snapping. Numbers in an
// observability dashboard often move slowly; the roll surfaces motion that
// would otherwise be invisible. When the delta direction flips (positive ↔
// negative), the TrendArrow component animates a directional bounce so the
// user sees *how* the metric moved, not just the new digit.

import CountUp from "./anim/CountUp";
import TrendArrow from "./anim/TrendArrow";

interface Props {
  title: string;
  value: string | number;
  unit?: string;
  delta?: number;
  hint?: string;
  accent?: "blue" | "green" | "amber" | "red" | "purple" | "warn" | "err";
}

const ACCENT_BG: Record<NonNullable<Props["accent"]>, string> = {
  blue: "from-blue-500/20 to-blue-500/0",
  green: "from-green-500/20 to-green-500/0",
  amber: "from-amber-500/20 to-amber-500/0",
  red: "from-red-500/20 to-red-500/0",
  purple: "from-purple-500/20 to-purple-500/0",
  warn: "from-amber-500/20 to-amber-500/0",
  err: "from-red-500/20 to-red-500/0",
};

const ACCENT_DOT: Record<NonNullable<Props["accent"]>, string> = {
  blue: "bg-blue-400",
  green: "bg-green-400",
  amber: "bg-amber-400",
  red: "bg-red-400",
  purple: "bg-purple-400",
  warn: "bg-amber-400",
  err: "bg-red-400",
};

export default function StatCard({
  title,
  value,
  unit,
  delta,
  hint,
  accent = "blue",
}: Props) {
  const numeric = typeof value === "number" ? value : Number(value) || 0;
  const isNumeric = typeof value === "number" || /^-?[\d,.]+$/.test(String(value));
  return (
    <div className="bg-grafana-panel border border-grafana-border rounded-lg p-4 relative overflow-hidden">
      <div
        className={
          "absolute inset-x-0 top-0 h-12 bg-gradient-to-b " +
          ACCENT_BG[accent] +
          " pointer-events-none"
        }
      />
      <div className="relative">
        <div className="flex items-center gap-2 text-[11px] text-grafana-muted uppercase tracking-wider">
          <span
            className={
              "inline-block w-1.5 h-1.5 rounded-full " + ACCENT_DOT[accent]
            }
          />
          {title}
        </div>
        <div className="mt-2 flex items-baseline gap-1.5">
          {isNumeric ? (
            <CountUp
              value={numeric}
              className="text-2xl font-semibold tabular-nums"
            />
          ) : (
            <span className="text-2xl font-semibold tabular-nums">{value}</span>
          )}
          {unit && (
            <span className="text-[12px] text-grafana-muted">{unit}</span>
          )}
        </div>
        <div className="mt-1 flex items-center justify-between text-[11px] text-grafana-muted">
          {typeof delta === "number" && <TrendArrow delta={delta} suffix="%" />}
          {hint && <span>{hint}</span>}
        </div>
      </div>
    </div>
  );
}
