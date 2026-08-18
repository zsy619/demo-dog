// A simple trace waterfall: each span is a horizontal bar whose length is
// proportional to its duration. Children are indented relative to their parent.

import type { Row } from "@/types/api";

interface Props {
  spans: Row[];
}

interface SpanLite {
  trace_id: string;
  span_id: string;
  parent_id?: string;
  name: string;
  service: string;
  start_time: string;
  duration_ms: number;
  status: string;
}

function asSpan(row: Row): SpanLite | null {
  if (
    typeof row.trace_id !== "string" ||
    typeof row.span_id !== "string" ||
    typeof row.name !== "string" ||
    typeof row.service !== "string" ||
    typeof row.start_time !== "string" ||
    typeof row.duration_ms !== "number"
  ) {
    return null;
  }
  return {
    trace_id: row.trace_id,
    span_id: row.span_id,
    parent_id: typeof row.parent_id === "string" ? row.parent_id : undefined,
    name: row.name,
    service: row.service,
    start_time: row.start_time,
    duration_ms: row.duration_ms,
    status: typeof row.status === "string" ? row.status : "unset",
  };
}

// Stable, well-spaced palette for service colours in waterfall rows.
const SERVICE_PALETTE = [
  "#3b82f6",
  "#10b981",
  "#a855f7",
  "#f59e0b",
  "#ef4444",
  "#06b6d4",
  "#84cc16",
  "#ec4899",
  "#14b8a6",
  "#f97316",
];

function serviceColor(svc: string): string {
  let hash = 0;
  for (let i = 0; i < svc.length; i++) {
    hash = (hash * 31 + svc.charCodeAt(i)) >>> 0;
  }
  return SERVICE_PALETTE[hash % SERVICE_PALETTE.length];
}

export default function TraceWaterfall({ spans }: Props) {
  if (spans.length === 0) {
    return (
      <div className="text-grafana-muted text-sm py-8 text-center">
        no traces in this view
      </div>
    );
  }

  // Group by trace_id, build a small waterfall per group.
  const groups = new Map<string, SpanLite[]>();
  for (const row of spans) {
    const s = asSpan(row);
    if (!s) continue;
    const g = groups.get(s.trace_id) ?? [];
    g.push(s);
    groups.set(s.trace_id, g);
  }

  return (
    <div className="space-y-3">
      {Array.from(groups.entries()).map(([traceId, list]) => {
        const valid = list.filter(Boolean);
        if (valid.length === 0) return null;
        const startTs = Math.min(
          ...valid.map((s) => new Date(s.start_time).getTime())
        );
        const endTs = Math.max(
          ...valid.map(
            (s) => new Date(s.start_time).getTime() + s.duration_ms
          )
        );
        const total = Math.max(endTs - startTs, 1);
        // Build depth map by walking parents
        const depth = new Map<string, number>();
        const findDepth = (s: SpanLite): number => {
          if (depth.has(s.span_id)) return depth.get(s.span_id)!;
          if (!s.parent_id) return 0;
          const parent = valid.find((p) => p.span_id === s.parent_id);
          if (!parent) return 0;
          return findDepth(parent) + 1;
        };
        valid.forEach((s) => depth.set(s.span_id, findDepth(s)));
        valid.sort((a, b) => {
          const da = depth.get(a.span_id) ?? 0;
          const db = depth.get(b.span_id) ?? 0;
          if (da !== db) return da - db;
          return (
            new Date(a.start_time).getTime() - new Date(b.start_time).getTime()
          );
        });

        // Service legend for this trace.
        const services = Array.from(new Set(valid.map((s) => s.service)));

        return (
          <div
            key={traceId}
            className="bg-grafana-panel border border-grafana-border rounded-md p-3"
          >
            <div className="flex items-center justify-between text-[11px] text-grafana-muted mb-2">
              <span className="font-mono">trace {traceId.slice(0, 12)}...</span>
              <span>{valid.length} spans · {total}ms</span>
            </div>
            <div className="flex flex-wrap gap-2 mb-2">
              {services.map((svc) => (
                <span
                  key={svc}
                  className="flex items-center gap-1 text-[10px] text-grafana-muted"
                >
                  <span
                    className="inline-block w-2 h-2 rounded-full"
                    style={{ background: serviceColor(svc) }}
                  />
                  {svc}
                </span>
              ))}
            </div>
            <div className="space-y-1">
              {valid.map((s) => {
                const start = new Date(s.start_time).getTime();
                const offset = ((start - startTs) / total) * 100;
                const width = Math.max((s.duration_ms / total) * 100, 0.4);
                const indent = (depth.get(s.span_id) ?? 0) * 16;
                const isError = s.status === "error";
                const base = isError
                  ? "bg-grafana-err/90 border border-grafana-err"
                  : "border border-white/10";
                return (
                  <div
                    key={s.span_id}
                    className="flex items-center text-xs gap-2 hover:bg-grafana-elev/40 rounded"
                  >
                    <div
                      className="font-mono text-grafana-text text-[10px] truncate flex items-center gap-1"
                      style={{ width: 200, paddingLeft: indent }}
                      title={s.service}
                    >
                      <span
                        className="inline-block w-1.5 h-1.5 rounded-full shrink-0"
                        style={{ background: serviceColor(s.service) }}
                      />
                      {s.name}
                    </div>
                    <div className="flex-1 h-3 bg-grafana-elev rounded relative">
                      <div
                        className={`absolute h-3 rounded ${base}`}
                        style={{
                          left: `${offset}%`,
                          width: `${width}%`,
                          opacity: isError ? 1 : 0.85,
                          backgroundColor: isError
                            ? undefined
                            : serviceColor(s.service),
                        }}
                        title={`${s.service} · ${s.duration_ms}ms · ${s.status}`}
                      />
                    </div>
                    <div className="text-[10px] text-grafana-muted w-16 text-right font-mono">
                      {s.duration_ms}ms
                    </div>
                    <div
                      className={`text-[9px] uppercase tracking-wider w-12 text-right ${
                        isError
                          ? "text-grafana-err"
                          : s.status === "ok"
                          ? "text-grafana-ok"
                          : "text-grafana-muted"
                      }`}
                    >
                      {s.status}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
