import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { SpanRecord, LogRecord } from "@/types/api";
import TraceWaterfall from "@/components/charts/TraceWaterfall";
import SearchBox from "@/components/SearchBox";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import { fmtTime, relativeTime, duration } from "@/lib/time";
import SeverityBadge from "@/components/SeverityBadge";
import Highlight from "@/components/Highlight";
import FadeIn from "@/components/anim/FadeIn";
import BlurText from "@/components/anim/BlurText";

interface Props {
  service: string;
  onServiceChange?: (s: string) => void;
  initialTraceId?: string;
}

interface TraceSummary {
  trace_id: string;
  service: string;
  spans: number;
  total_ms: number;
  max_ms: number;
  root: string;
  has_error: boolean;
}

export default function Traces({ service, onServiceChange, initialTraceId }: Props) {
  const [spans, setSpans] = useState<SpanRecord[]>([]);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string | null>(initialTraceId || null);
  const [detail, setDetail] = useState<SpanRecord[] | null>(null);
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = async () => {
    try {
      const r = await api.query({
        type: "traces",
        service,
        search,
        limit: 200,
      });
      setSpans((r.rows as unknown as SpanRecord[]) ?? []);
      setErr(null);
    } catch (e) {
      setErr(e as Error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setLoading(true);
    reload();
    const id = window.setInterval(reload, 6000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [service, search]);

  // Aggregate per-trace.
  const traces: TraceSummary[] = useMemo(() => {
    const map = new Map<string, { spans: SpanRecord[]; svc: string; hasErr: boolean }>();
    for (const s of spans) {
      if (!s.trace_id) continue;
      const cur = map.get(s.trace_id) ?? { spans: [], svc: s.service, hasErr: false };
      cur.spans.push(s);
      if (s.status === "error") cur.hasErr = true;
      map.set(s.trace_id, cur);
    }
    return Array.from(map.entries()).map(([id, v]) => {
      const root = v.spans.find((s) => !s.parent_id);
      return {
        trace_id: id,
        service: v.svc,
        spans: v.spans.length,
        total_ms: v.spans.reduce((a, b) => a + b.duration_ms, 0),
        max_ms: Math.max(...v.spans.map((s) => s.duration_ms)),
        root: root?.name ?? "(no root)",
        has_error: v.hasErr,
      };
    });
  }, [spans]);

  // Load detail.
  useEffect(() => {
    if (!selected) {
      setDetail(null);
      return;
    }
    api.trace(selected).then((r) => setDetail(r.spans)).catch(() => setDetail(null));
  }, [selected]);

  return (
    <FadeIn className="p-4 space-y-3">
      {err && <ErrorBox error={err} onRetry={reload} />}

      <div className="bg-grafana-panel border border-grafana-border rounded-lg p-3 flex flex-wrap items-center gap-2 text-xs">
        <input
          value={service}
          onChange={(e) => onServiceChange?.(e.target.value)}
          placeholder="service (all)"
          className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 w-40 focus:outline-none focus:border-grafana-blue"
        />
        <div className="w-64">
          <SearchBox
            value={search}
            onChange={setSearch}
            placeholder="search span name"
          />
        </div>
        <button
          onClick={reload}
          className="bg-grafana-elev border border-grafana-border rounded px-3 py-1 hover:bg-grafana-elev2"
        >
          ⟳ Refresh
        </button>
        <span className="text-grafana-muted ml-auto">
          {traces.length} traces · {spans.length} spans
        </span>
      </div>

      <div className="grid grid-cols-12 gap-3">
        <div className="col-span-12 lg:col-span-4 bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
          <div className="px-3 py-2 border-b border-grafana-border text-[11px] uppercase tracking-wider text-grafana-muted">
            Traces
          </div>
          {loading && traces.length === 0 ? (
            <div className="p-3"><Skeleton rows={4} cols={3} /></div>
          ) : traces.length === 0 ? (
            <div className="text-grafana-muted italic text-center py-8">
              No traces match these filters.
            </div>
          ) : (
            <ul className="divide-y divide-grafana-border max-h-[640px] overflow-y-auto scrollbar-thin">
              {(() => {
                // Reference max for relative latency bars within the current page.
                const refMax = Math.max(
                  1,
                  ...traces.map((t) => t.max_ms)
                );
                return traces.map((t) => (
                <li
                  key={t.trace_id}
                  className={`px-3 py-2 cursor-pointer hover:bg-grafana-elev/50 ${
                    selected === t.trace_id ? "bg-grafana-elev" : ""
                  }`}
                  onClick={() => setSelected(t.trace_id)}
                >
                  <div className="flex items-center justify-between text-[12px]">
                    <span className="font-mono text-grafana-blue truncate" title={t.trace_id}>
                      {t.trace_id.slice(0, 16)}
                    </span>
                    {t.has_error ? (
                      <span className="text-grafana-err text-[10px]">● err</span>
                    ) : (
                      <span className="text-grafana-ok text-[10px]">● ok</span>
                    )}
                  </div>
                  <div className="text-[11px] text-grafana-muted mt-0.5 truncate" title={t.root}>
                    {t.root}
                  </div>
                  <div className="text-[10px] text-grafana-muted font-mono mt-0.5 flex justify-between">
                    <span>{t.spans} spans</span>
                    <span>max {duration(t.max_ms)}</span>
                  </div>
                  <div className="mt-1 h-1 bg-grafana-elev rounded overflow-hidden">
                    <div
                      className={`h-full ${t.has_error ? "bg-grafana-err" : "bg-grafana-accent"}`}
                      style={{ width: `${Math.min(100, (t.max_ms / refMax) * 100)}%` }}
                      title={`latency ${duration(t.max_ms)} · ref ${duration(refMax)}`}
                    />
                  </div>
                </li>
                ));
              })()}
            </ul>
          )}
        </div>

        <div className="col-span-12 lg:col-span-8 space-y-3">
          {selected ? (
            <TraceDetail
              traceId={selected}
              spans={detail}
              onClose={() => setSelected(null)}
              onOpenLogs={(tid) => {
                if (typeof window !== "undefined") {
                  window.location.hash = `#/logs?trace_id=${encodeURIComponent(tid)}`;
                }
              }}
            />
          ) : (
            <div className="bg-grafana-panel border border-grafana-border rounded-lg p-6 text-grafana-muted text-center text-sm">
              ← select a trace to inspect
            </div>
          )}
        </div>
      </div>
    </FadeIn>
  );
}

function TraceDetail({
  traceId,
  spans,
  onClose,
  onOpenLogs,
}: {
  traceId: string;
  spans: SpanRecord[] | null;
  onClose: () => void;
  onOpenLogs: (traceId: string) => void;
}) {
  if (!spans) {
    return (
      <div className="bg-grafana-panel border border-grafana-border rounded-lg p-6 text-center text-grafana-muted">
        loading trace {traceId}…
      </div>
    );
  }
  if (spans.length === 0) {
    return (
      <div className="bg-grafana-panel border border-grafana-border rounded-lg p-6 text-center text-grafana-muted">
        trace {traceId} not found
      </div>
    );
  }
  const total = Math.max(...spans.map((s) => s.duration_ms), 1);
  const start = Math.min(...spans.map((s) => new Date(s.start_time).getTime()));

  return (
    <div className="space-y-3">
      <div className="bg-grafana-panel border border-grafana-border rounded-lg">
        <div className="px-4 py-2 border-b border-grafana-border flex items-center justify-between">
          <div>
            <div className="text-[13px] font-semibold flex items-center gap-2">
              Trace
              <span className="font-mono text-grafana-blue">{traceId.slice(0, 16)}</span>
              <button
                onClick={() => {
                  navigator.clipboard?.writeText(traceId).then(
                    () => {
                      // Tiny inline feedback: swap tooltip text via state.
                      const el = document.activeElement as HTMLElement | null;
                      if (el) {
                        const original = el.getAttribute("data-tip") ?? "";
                        el.setAttribute("data-tip", "copied!");
                        el.setAttribute("title", "copied!");
                        window.setTimeout(() => {
                          el.setAttribute("title", original || "copy trace id");
                          el.removeAttribute("data-tip");
                        }, 1200);
                      }
                    },
                    () => {
                      /* ignore */
                    }
                  );
                }}
                className="text-grafana-muted hover:text-grafana-text text-xs"
                title="copy trace id"
              >
                ⎘
              </button>
            </div>
            <div className="text-[11px] text-grafana-muted">
              {spans.length} spans · {duration(total)} total
            </div>
          </div>
          <DurationHistogram spans={spans} />
          <button
            onClick={onClose}
            className="text-grafana-muted hover:text-grafana-text text-sm"
          >
            × close
          </button>
        </div>
        <div className="p-4">
          <TraceWaterfall spans={spans as any} />
        </div>
      </div>

      <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[12px] text-grafana-muted">
          Spans
        </div>
        <table className="w-full text-sm">
          <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
            <tr>
              <th className="px-3 py-2 text-left">Service</th>
              <th className="px-3 py-2 text-left">Name</th>
              <th className="px-3 py-2 text-right">Duration</th>
              <th className="px-3 py-2 text-right">Start</th>
              <th className="px-3 py-2 text-left">Status</th>
              <th className="px-3 py-2 text-left">Attrs</th>
            </tr>
          </thead>
          <tbody>
            {spans.map((s) => {
              const offset = new Date(s.start_time).getTime() - start;
              return (
                <tr key={s.span_id} className="border-t border-grafana-border">
                  <td className="px-3 py-1.5 text-grafana-accent">{s.service}</td>
                  <td className="px-3 py-1.5">
                    {s.name}
                    {s.parent_id && (
                      <span className="ml-1 text-grafana-muted text-[10px]">
                        ← {s.parent_id.slice(0, 6)}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-1.5 text-right font-mono">
                    {duration(s.duration_ms)}
                  </td>
                  <td className="px-3 py-1.5 text-right text-[11px] text-grafana-muted">
                    +{offset.toFixed(0)}ms
                  </td>
                  <td className="px-3 py-1.5">
                    <span
                      className={
                        s.status === "error"
                          ? "text-grafana-err"
                          : s.status === "ok"
                          ? "text-grafana-ok"
                          : "text-grafana-muted"
                      }
                    >
                      {s.status}
                    </span>
                  </td>
                  <td className="px-3 py-1.5 text-[11px] font-mono text-grafana-muted">
                    {s.attributes
                      ? Object.entries(s.attributes)
                          .slice(0, 3)
                          .map(([k, v]) => `${k}=${v}`)
                          .join(" ")
                      : "—"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <TraceLogs traceId={traceId} onOpenLogs={onOpenLogs} />
    </div>
  );
}

function TraceLogs({
  traceId,
  onOpenLogs,
}: {
  traceId: string;
  onOpenLogs: (traceId: string) => void;
}) {
  const [logs, setLogs] = useState<LogRecord[]>([]);
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .query({ type: "logs", trace_id: traceId, limit: 50 })
      .then((r) => {
        if (cancelled) return;
        setLogs((r.rows as unknown as LogRecord[]) ?? []);
        setLoading(false);
      })
      .catch((e) => {
        if (cancelled) return;
        setErr(e as Error);
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [traceId]);

  return (
    <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
      <div className="px-4 py-2 border-b border-grafana-border text-[12px] flex items-center justify-between">
        <span className="text-grafana-muted">Logs for this trace</span>
        <span className="flex items-center gap-2">
          <span className="text-grafana-muted">{logs.length}</span>
          <button
            onClick={() => onOpenLogs(traceId)}
            className="text-[11px] text-grafana-blue hover:underline"
            title="Open in Logs page with this trace filter"
          >
            open in logs ↗
          </button>
        </span>
      </div>
      {loading ? (
        <div className="p-3">
          <Skeleton rows={2} />
        </div>
      ) : err ? (
        <ErrorBox error={err} />
      ) : logs.length === 0 ? (
        <div className="text-grafana-muted italic text-xs px-4 py-6 text-center">
          No log records found for this trace id.
        </div>
      ) : (
        <table className="w-full text-sm">
          <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
            <tr>
              <th className="px-3 py-2 text-left">Time</th>
              <th className="px-3 py-2 text-left">Service</th>
              <th className="px-3 py-2 text-left">Level</th>
              <th className="px-3 py-2 text-left">Message</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((r, i) => (
              <tr key={i} className="border-t border-grafana-border">
                <td className="px-3 py-1.5 text-[11px] text-grafana-muted whitespace-nowrap">
                  {fmtTime(new Date(r.timestamp).getTime())}
                </td>
                <td className="px-3 py-1.5 text-grafana-accent whitespace-nowrap">
                  {r.service}
                </td>
                <td className="px-3 py-1.5">
                  <SeverityBadge value={r.severity} />
                </td>
                <td className="px-3 py-1.5 font-mono text-[12px]">
                  <Highlight text={r.body ?? ""} query="" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}


// Tiny horizontal histogram that visualises how span durations are distributed
// within the current trace. Uses 8 buckets across the [min, max] range so users
// can quickly spot a slow-tail span without parsing the table.
function DurationHistogram({ spans }: { spans: SpanRecord[] }) {
  if (spans.length === 0) return null;
  const durs = spans.map((s) => Math.max(1, s.duration_ms));
  const min = Math.min(...durs);
  const max = Math.max(...durs);
  if (max === min) {
    return (
      <div className="text-[10px] text-grafana-muted font-mono">
        {spans.length} × {duration(max)}
      </div>
    );
  }
  const BUCKETS = 8;
  const buckets = new Array(BUCKETS).fill(0) as number[];
  for (const d of durs) {
    const idx = Math.min(
      BUCKETS - 1,
      Math.floor(((d - min) / (max - min)) * BUCKETS)
    );
    buckets[idx]++;
  }
  const peak = Math.max(...buckets);
  return (
    <div className="flex items-end gap-1 h-12" title="span duration distribution">
      {buckets.map((c, i) => {
        const h = peak > 0 ? (c / peak) * 100 : 0;
        const isError = spans[i]?.status === "error";
        const color = isError
          ? "bg-grafana-err/80"
          : c > 0
          ? "bg-grafana-accent/80"
          : "bg-grafana-elev";
        return (
          <div
            key={i}
            className={`w-3 ${color} rounded-sm transition-all`}
            style={{ height: `${Math.max(h, 4)}%` }}
            title={`${duration(min + ((max - min) * i) / BUCKETS)}-${duration(min + ((max - min) * (i + 1)) / BUCKETS)} · ${c} span${c === 1 ? "" : "s"}`}
          />
        );
      })}
      <div className="text-[10px] text-grafana-muted ml-2 self-end">
        {duration(min)}–{duration(max)}
      </div>
    </div>
  );
}
