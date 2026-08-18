import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { LogRecord, Severity } from "@/types/api";
import SeverityBadge from "@/components/SeverityBadge";
import SearchBox from "@/components/SearchBox";
import { useHashState } from "@/hooks/useHashState";
import TimeRangePicker from "@/components/TimeRangePicker";
import Highlight from "@/components/Highlight";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import { fmtTime, relativeTime, sinceMs } from "@/lib/time";
import FadeIn from "@/components/anim/FadeIn";
import Stagger from "@/components/anim/Stagger";
import { useStream } from "@/hooks/useStream";
import { toast } from "@/components/Toast";

interface Props {
  service: string;
  onServiceChange?: (s: string) => void;
  onOpenTrace?: (traceId: string) => void;
  initialTraceId?: string;
}

const SEVERITIES: Severity[] = ["TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"];

function rankSeverity(s: string): number {
  return SEVERITIES.indexOf(s as Severity);
}

// ParsedQuery turns the free-form search string into a structured filter.
// Tokens recognised:
//   service=<name>            exact service filter (multiple accumulate)
//   sev / severity=ERROR      exact severity match
//   severity>=WARN            at-least severity (>=)
//   severity<=INFO            at-most severity (<=)
//   trace / trace_id=<id>     trace id filter
//   host=<x>                  label match (any key other than the above is a label)
//   any other free word       substring search across log body
// Quoted strings ("...") preserve whitespace inside the value.
export interface ParsedQuery {
  service: string[];
  severity?: string;
  severityAtLeast?: string;
  severityAtMost?: string;
  traceId?: string;
  labels: Record<string, string>;
  body: string;
}

export function parseSearchQuery(input: string): ParsedQuery {
  const out: ParsedQuery = {
    service: [],
    labels: {},
    body: "",
  };
  const bodyParts: string[] = [];
  // Split on whitespace but keep quoted strings intact.
  const tokens: string[] = [];
  let buf = "";
  let inQ = false;
  for (let i = 0; i < input.length; i++) {
    const c = input[i];
    if (c === '"') {
      inQ = !inQ;
      continue;
    }
    if (!inQ && /\s/.test(c)) {
      if (buf) {
        tokens.push(buf);
        buf = "";
      }
    } else {
      buf += c;
    }
  }
  if (buf) tokens.push(buf);

  for (const tok of tokens) {
    const m = tok.match(/^([a-zA-Z_]+)(>=|<=|=)(.*)$/);
    if (m) {
      const [, key, op, value] = m;
      const k = key.toLowerCase();
      const v = value.trim();
      if (k === "service") {
        if (v && !out.service.includes(v)) out.service.push(v);
        continue;
      }
      if (k === "sev" || k === "severity" || k === "level") {
        const norm = normalizeSeverity(v);
        if (!norm) continue;
        if (op === ">=") out.severityAtLeast = norm;
        else if (op === "<=") out.severityAtMost = norm;
        else out.severity = norm;
        continue;
      }
      if (k === "trace" || k === "trace_id") {
        out.traceId = v;
        continue;
      }
      // Everything else is a label key=value pair.
      if (v) out.labels[key] = v;
      continue;
    }
    // bare token: substring search against body
    bodyParts.push(tok);
  }
  out.body = bodyParts.join(" ");
  return out;
}

function normalizeSeverity(s: string): Severity | null {
  const u = s.toUpperCase().trim();
  // Accept common alternates: warning → WARN, err → ERROR, panic/fatal → FATAL.
  const map: Record<string, Severity> = {
    TRACE: "TRACE",
    DEBUG: "DEBUG",
    INFO: "INFO",
    INFORMATION: "INFO",
    NOTICE: "INFO",
    WARN: "WARN",
    WARNING: "WARN",
    ERR: "ERROR",
    ERROR: "ERROR",
    CRITICAL: "ERROR",
    FATAL: "FATAL",
    PANIC: "FATAL",
    EMERG: "FATAL",
    ALERT: "FATAL",
  };
  return map[u] ?? (SEVERITIES.includes(u as Severity) ? (u as Severity) : null);
}

// SeverityMatch evaluates whether a row severity satisfies the parsed query.
function severityMatches(sev: string, q: ParsedQuery): boolean {
  if (q.severity && sev !== q.severity) return false;
  if (q.severityAtLeast && rankSeverity(sev) < rankSeverity(q.severityAtLeast))
    return false;
  if (q.severityAtMost && rankSeverity(sev) > rankSeverity(q.severityAtMost))
    return false;
  return true;
}

function LivePulse({
  live,
  lastReceived,
  rate,
}: {
  live: boolean;
  lastReceived: number | null;
  rate: number;
}) {
  if (!live) {
    return (
      <span className="text-[11px] text-grafana-muted px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40">
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-grafana-muted mr-1" />
        live off
      </span>
    );
  }
  const stale =
    lastReceived === null || Date.now() - lastReceived > 5000;
  const color = stale
    ? "bg-grafana-muted"
    : rate > 5
    ? "bg-grafana-ok"
    : "bg-grafana-accent";
  return (
    <span className="text-[11px] px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 flex items-center gap-2">
      <span className={`inline-block w-1.5 h-1.5 rounded-full ${color} ${stale ? "" : "animate-pulse-soft"}`} />
      <span className={stale ? "text-grafana-muted" : "text-grafana-ok"}>
        {stale ? "idle" : "live"}
      </span>
      <span className="text-grafana-border">·</span>
      <span className="text-grafana-muted">
        <b className="text-grafana-text tabular-nums">{rate.toFixed(1)}</b> ev/s
      </span>
    </span>
  );
}

export default function Logs({ service, onServiceChange, onOpenTrace, initialTraceId }: Props) {
  const [rows, setRows] = useState<LogRecord[]>([]);
  // Filter state lives in the URL hash so it survives Cmd+R and produces
  // shareable links to specific log views (e.g. "all ERROR logs from checkout
  // in the last hour with host=pod-33").
  const [severity, setSeverity] = useHashState("severity", "");
  const [atLeastStr, setAtLeastStr] = useHashState("atLeast", "");
  const atLeast = atLeastStr === "1";
  const setAtLeast = (b: boolean) => setAtLeastStr(b ? "1" : "");
  const [search, setSearch] = useHashState("q", "");
  const [range, setRange] = useHashState("range", "15m");
  const [labelsText, setLabelsText] = useHashState("labels", "");
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);
  const [live, setLive] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [traceFilter, setTraceFilter] = useState<string | undefined>(initialTraceId);

  const liveEvents = useStream({ kinds: ["log"], max: 50 });
  const tailRef = useRef<HTMLDivElement>(null);

  // Event-rate accounting: count WS events received per second and remember
  // the timestamp of the last live event so the toolbar can show a stale dot.
  const [rate, setRate] = useState<number>(0);
  const [lastReceived, setLastReceived] = useState<number | null>(null);
  const lastCountRef = useRef(0);
  const ringBufferRef = useRef<Array<{ ts: number; per: number }>>([]);
  const updateRate = () => {
    const now = Date.now();
    ringBufferRef.current = ringBufferRef.current.filter((b) => b.ts >= now - 5000);
    const buckets = ringBufferRef.current;
    if (buckets.length === 0) {
      setRate(0);
      return;
    }
    const span = Math.max(1, (now - buckets[0].ts) / 1000);
    const total = buckets.reduce((a, b) => a + b.per, 0);
    setRate(total / span);
  };
  useEffect(() => {
    if (liveEvents.length === 0) return;
    const latest = liveEvents[0]?.timestamp ?? null;
    if (latest !== null) setLastReceived(latest);
    const delta = Math.max(0, liveEvents.length - lastCountRef.current);
    lastCountRef.current = liveEvents.length;
    if (delta > 0) {
      ringBufferRef.current.push({ ts: Date.now(), per: delta });
      updateRate();
    }
  }, [liveEvents]);
  // Periodically age out old buckets so the rate decays toward 0 on silence.
  useEffect(() => {
    if (!live) return;
    const id = window.setInterval(updateRate, 1500);
    return () => window.clearInterval(id);
  }, [live]);

  const labels = useMemo(() => {
    const out: Record<string, string> = {};
    for (const piece of labelsText.split(/[\s,]+/).filter(Boolean)) {
      const [k, v] = piece.split("=");
      if (k && v) out[k] = v;
    }
    return out;
  }, [labelsText]);

  // Sync the trace filter from the URL when it changes (e.g. user navigates
  // from Traces with ?trace_id=...).
  useEffect(() => {
    setTraceFilter(initialTraceId);
  }, [initialTraceId]);

  // Structured-search parser: extracts tokens like `service=foo severity>=WARN trace=...`
  // out of the free-form search string so users can express complex filters in
  // one box (Loki/LogQL-inspired). Bare words fall through to body substring.
  const parsed = useMemo(() => parseSearchQuery(search), [search]);

  // Build the final severity string from parsed tokens + UI controls.
  const effectiveSeverity = useMemo(() => {
    if (parsed.severityAtLeast) return `>=${parsed.severityAtLeast}`;
    if (parsed.severity) return parsed.severity;
    return atLeast && severity ? `>=${severity}` : severity;
  }, [parsed, atLeast, severity]);

  const reload = async () => {
    try {
      // Merge labels from `key=value` tokens in the search with the manual labels box.
      const mergedLabels = { ...labels, ...parsed.labels };
      // Combine the dropdown selection with any `service=` tokens from the
      // search box into a deduped list. We then fire one query per service
      // and merge the results so multi-service filters actually work end to
      // end (the backend currently only honours a single `service=` value).
      const allServices = Array.from(
        new Set(
          [service, ...parsed.service].filter((s): s is string => Boolean(s))
        )
      );
      const servicesToQuery = allServices.length > 0 ? allServices : [""];
      const partials = await Promise.all(
        servicesToQuery.map((svc) =>
          api
            .query({
              type: "logs",
              service: svc || undefined,
              severity:
                Array.isArray(effectiveSeverity)
                  ? effectiveSeverity[0]
                  : effectiveSeverity,
              search: parsed.body,
              since: sinceMs(range as any),
              labels: mergedLabels,
              trace_id: parsed.traceId ?? traceFilter,
              limit: 300,
            })
            .catch(() => ({ rows: [] as unknown[] }))
        )
      );
      const merged = new Map<string, unknown>();
      for (const p of partials) {
        for (const row of (p.rows ?? []) as Array<Record<string, unknown>>) {
          const key = `${row.timestamp ?? ""}|${row.service ?? ""}|${row.body ?? ""}`;
          if (!merged.has(key)) merged.set(key, row);
        }
      }
      setRows(Array.from(merged.values()) as unknown as LogRecord[]);
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
    const id = window.setInterval(() => {
      if (live) reload();
    }, 4000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [service, severity, atLeast, search, range, labelsText, live, traceFilter, parsed.body]);

  // Live tail: prepend new WS events.
  useEffect(() => {
    if (!live || liveEvents.length === 0) return;
    setRows((prev) => {
      const seen = new Set(prev.map((r) => `${r.timestamp}|${r.body}|${r.service}`));
      const fresh = liveEvents
        .map(
          (ev): LogRecord => ({
            timestamp: new Date(ev.timestamp).toISOString(),
            service: ev.service,
            severity: (ev.status as Severity) ?? "INFO",
            body: ev.body ?? "",
            trace_id: ev.trace_id,
            span_id: ev.span_id,
          })
        )
        .filter((r) => !seen.has(`${r.timestamp}|${r.body}|${r.service}`))
        .filter((r) => (service ? r.service === service : true))
        .filter((r) => {
          if (!severity) return true;
          if (atLeast) return rankSeverity(r.severity) >= rankSeverity(severity);
          return r.severity === severity;
        });
      if (fresh.length === 0) return prev;
      return [...fresh, ...prev].slice(0, 300);
    });
    if (autoScroll && tailRef.current) tailRef.current.scrollTop = 0;
  }, [liveEvents, live, service, severity, atLeast, autoScroll]);

  const exportCSV = () => {
    const url = api.exportUrl({ type: "logs", service, severity, search, since: sinceMs(range as any), labels, limit: 5000, format: "csv" });
    window.open(url, "_blank");
  };

  const exportJSON = () => {
    const url = api.exportUrl({ type: "logs", service, severity, search, since: sinceMs(range as any), labels, limit: 5000, format: "json" });
    window.open(url, "_blank");
  };

  const seed = async () => {
    const svc = service || "demo";
    try {
      const r = await api.seed(svc, 5);
      toast(`seeded ${r.seeded} events for ${r.service}`, "success");
      reload();
    } catch (e) {
      toast(`seed failed: ${(e as Error).message}`, "error");
    }
  };

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
        <SeverityChips value={severity} onChange={setSeverity} atLeast={atLeast} onAtLeastChange={setAtLeast} />
        <div className="w-64">
          <SearchBox
            value={search}
            onChange={setSearch}
            placeholder="search · service=foo severity>=WARN trace=…"
          />
        </div>
        <TimeRangePicker value={range} onChange={setRange} />
        <input
          value={labelsText}
          onChange={(e) => setLabelsText(e.target.value)}
          placeholder="labels: env=prod region=us-east"
          className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 w-64 focus:outline-none focus:border-grafana-blue"
        />
        <label className="text-grafana-muted flex items-center gap-1">
          <input
            type="checkbox"
            checked={live}
            onChange={(e) => setLive(e.target.checked)}
          />
          live
        </label>
        <label className="text-grafana-muted flex items-center gap-1">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
          />
          auto-scroll
        </label>
        <button
          onClick={reload}
          className="bg-grafana-elev border border-grafana-border rounded px-3 py-1 hover:bg-grafana-elev2"
        >
          ⟳ Refresh
        </button>
        <button
          onClick={exportCSV}
          className="bg-grafana-elev border border-grafana-border rounded px-3 py-1 hover:bg-grafana-elev2"
        >
          ⬇ CSV
        </button>
        <button
          onClick={exportJSON}
          className="bg-grafana-elev border border-grafana-border rounded px-3 py-1 hover:bg-grafana-elev2"
        >
          ⬇ JSON
        </button>
        <button
          onClick={seed}
          className="bg-grafana-blue/20 border border-grafana-blue/40 rounded px-3 py-1 hover:bg-grafana-blue/30 text-grafana-blue"
        >
          + seed
        </button>
        <span className="ml-auto flex items-center gap-2">
          <LivePulse
            live={live}
            lastReceived={lastReceived}
            rate={rate}
          />
        </span>
      </div>

      {/* Parsed-tokens hint: show what was extracted from the search string. */}
      {(parsed.service.length > 0 ||
        parsed.severity ||
        parsed.severityAtLeast ||
        parsed.severityAtMost ||
        parsed.traceId ||
        Object.keys(parsed.labels).length > 0) && (
        <div className="flex items-center gap-1.5 text-[11px] text-grafana-muted flex-wrap">
          <span className="text-grafana-muted">parsed:</span>
          {parsed.service.map((s) => (
            <span
              key={s}
              className="bg-grafana-blue/15 text-grafana-blue px-1.5 py-0.5 rounded font-mono"
            >
              service={s}
            </span>
          ))}
          {parsed.severity && (
            <span className="bg-grafana-purple/15 text-grafana-purple px-1.5 py-0.5 rounded font-mono">
              severity={parsed.severity}
            </span>
          )}
          {parsed.severityAtLeast && (
            <span className="bg-grafana-purple/15 text-grafana-purple px-1.5 py-0.5 rounded font-mono">
              severity&gt;={parsed.severityAtLeast}
            </span>
          )}
          {parsed.severityAtMost && (
            <span className="bg-grafana-purple/15 text-grafana-purple px-1.5 py-0.5 rounded font-mono">
              severity&lt;={parsed.severityAtMost}
            </span>
          )}
          {parsed.traceId && (
            <span className="bg-grafana-accent/15 text-grafana-accent px-1.5 py-0.5 rounded font-mono">
              trace={parsed.traceId.slice(0, 12)}
            </span>
          )}
          {Object.entries(parsed.labels).map(([k, v]) => (
            <span
              key={k}
              className="bg-grafana-elev border border-grafana-border px-1.5 py-0.5 rounded font-mono"
            >
              {k}={v}
            </span>
          ))}
        </div>
      )}

      {traceFilter && (
        <div className="bg-grafana-blue/10 border border-grafana-blue/30 rounded px-3 py-1.5 flex items-center gap-2 text-xs">
          <span className="text-grafana-blue font-semibold">trace filter:</span>
          <span className="font-mono text-grafana-text truncate">
            {traceFilter}
          </span>
          <button
            className="ml-auto text-grafana-muted hover:text-grafana-text"
            onClick={() => {
              // Clear the trace filter without touching other filter state. We
              // surgically remove `trace_id` from the existing hash so the
              // user keeps their severity/range/search.
              if (typeof window !== "undefined") {
                const h = window.location.hash;
                const qIdx = h.indexOf("?");
                const base = qIdx >= 0 ? h.slice(0, qIdx) : h;
                const params = new URLSearchParams(qIdx >= 0 ? h.slice(qIdx + 1) : "");
                params.delete("trace_id");
                const q = params.toString();
                window.history.replaceState(null, "", q ? `${base}?${q}` : base);
                window.dispatchEvent(new HashChangeEvent("hashchange"));
              }
              setTraceFilter(undefined);
            }}
            title="Clear trace filter"
          >
            clear ×
          </button>
        </div>
      )}

      <div className="grid grid-cols-12 gap-3">
        <div className="col-span-12 lg:col-span-9 bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
          <div className="px-3 py-2 border-b border-grafana-border text-[12px] text-grafana-muted flex justify-between">
            <span>
              <b className="text-grafana-text">{rows.length}</b> records
              {search && <> · matching "{search}"</>}
              {Object.keys(labels).length > 0 && (
                <> · labels: {Object.entries(labels).map(([k, v]) => `${k}=${v}`).join(", ")}</>
              )}
            </span>
            <span>showing last {range}</span>
          </div>
          <div ref={tailRef} className="max-h-[600px] overflow-y-auto scrollbar-thin">
            {loading && rows.length === 0 ? (
              <div className="p-4">
                <Skeleton rows={4} cols={3} />
              </div>
            ) : rows.length === 0 ? (
              <div className="text-center text-grafana-muted italic py-8">
                No logs match these filters.
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider sticky top-0">
                  <tr>
                    <th className="px-3 py-2 text-left">Time</th>
                    <th className="px-3 py-2 text-left">Service</th>
                    <th className="px-3 py-2 text-left">Level</th>
                    <th className="px-3 py-2 text-left">Message</th>
                    <th className="px-3 py-2 text-left">Trace</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((r, i) => (
                    <tr key={i} className="border-t border-grafana-border hover:bg-grafana-elev/50">
                      <td className="px-3 py-1 font-mono text-[11px] whitespace-nowrap">
                        <span title={r.timestamp}>{relativeTime(r.timestamp)}</span>
                      </td>
                      <td className="px-3 py-1 text-grafana-accent whitespace-nowrap">{r.service}</td>
                      <td className="px-3 py-1">
                        <SeverityBadge value={r.severity} />
                      </td>
                      <td className="px-3 py-1 font-mono text-[12px]">
                        <Highlight text={r.body ?? ""} query={search} />
                        {r.attributes && Object.keys(r.attributes).length > 0 && (
                          <div className="text-[10px] text-grafana-muted mt-0.5">
                            {Object.entries(r.attributes)
                              .slice(0, 6)
                              .map(([k, v]) => `${k}=${v}`)
                              .join(" ")}
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-1 font-mono text-[11px]">
                        {r.trace_id ? (
                          <button
                            className="text-grafana-blue hover:underline"
                            title={`Open trace ${r.trace_id}`}
                            onClick={() => {
                              if (onOpenTrace) onOpenTrace(r.trace_id!);
                              else if (r.trace_id) {
                                window.location.hash = `#/traces?trace_id=${r.trace_id}`;
                              }
                            }}
                          >
                            {r.trace_id.slice(0, 12)}
                          </button>
                        ) : (
                          <span className="text-grafana-muted">—</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        <div className="col-span-12 lg:col-span-3 bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
          <div className="px-3 py-2 border-b border-grafana-border text-[11px] uppercase tracking-wider text-grafana-muted flex justify-between">
            <span>Live stream</span>
            <span className="text-grafana-ok">● WS</span>
          </div>
          <div className="max-h-[600px] overflow-y-auto scrollbar-thin">
            {liveEvents.length === 0 ? (
              <div className="text-grafana-muted italic text-xs p-3">waiting…</div>
            ) : (
              <ul className="divide-y divide-grafana-border font-mono text-[11px]">
                {liveEvents.map((ev, i) => (
                  <li key={i} className="px-3 py-1.5">
                    <div className="flex items-center gap-2">
                      <span className="text-grafana-muted whitespace-nowrap">
                        {fmtTime(ev.timestamp)}
                      </span>
                      <SeverityBadge value={(ev.status as Severity) ?? "INFO"} />
                      <span className="text-grafana-accent truncate">{ev.service}</span>
                    </div>
                    <div className="truncate mt-0.5" title={ev.body}>
                      {ev.body}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>
    </FadeIn>
  );
}


// SeverityChips renders the six canonical severities as inline chips with
// their respective colour swatch and supports an 'at-least' mode toggle so
// the user can express both `severity=ERROR` and `severity>=WARN` from the UI.
function SeverityChips({
  value,
  onChange,
  atLeast,
  onAtLeastChange,
}: {
  value: string;
  onChange: (v: string) => void;
  atLeast: boolean;
  onAtLeastChange: (b: boolean) => void;
}) {
  const PALETTE: Record<Severity, string> = {
    TRACE: "#6b7280",
    DEBUG: "#6b7280",
    INFO:  "#3b82f6",
    WARN:  "#f59e0b",
    ERROR: "#ef4444",
    FATAL: "#dc2626",
  };
  return (
    <div className="flex items-center gap-1" title="Filter by severity">
      <button
        onClick={() => onChange("")}
        className={`px-1.5 py-1 rounded text-[10px] border transition-colors ${
          value === ""
            ? "bg-grafana-accent/15 border-grafana-accent text-grafana-accent"
            : "bg-grafana-elev border-grafana-border text-grafana-muted hover:text-grafana-text"
        }`}
      >
        all
      </button>
      {SEVERITIES.map((s) => {
        const isOn = value === s;
        const c = PALETTE[s];
        return (
          <button
            key={s}
            onClick={() => onChange(isOn ? "" : s)}
            className={`flex items-center gap-1 px-1.5 py-1 rounded text-[10px] border transition-colors ${
              isOn
                ? "border-current"
                : "bg-grafana-elev border-grafana-border text-grafana-muted hover:text-grafana-text"
            }`}
            style={isOn ? { background: c + "26", color: c } : undefined}
          >
            <span
              className="inline-block w-2 h-2 rounded-full"
              style={{ background: c }}
            />
            {s}
          </button>
        );
      })}
      <label
        className={`ml-1 px-1.5 py-1 rounded text-[10px] border cursor-pointer flex items-center gap-1 transition-colors ${
          atLeast
            ? "bg-grafana-purple/15 border-grafana-purple text-grafana-purple"
            : "bg-grafana-elev border-grafana-border text-grafana-muted"
        }`}
        title="Show this severity and above (>=)"
      >
        <input
          type="checkbox"
          checked={atLeast}
          onChange={(e) => onAtLeastChange(e.target.checked)}
          disabled={!value}
          className="hidden"
        />
        ≥
      </label>
    </div>
  );
}
