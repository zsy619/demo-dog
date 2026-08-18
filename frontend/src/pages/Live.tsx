import { useEffect, useMemo, useRef, useState } from "react";
import { useStream } from "@/hooks/useStream";
import type { StreamEvent } from "@/types/api";
import SeverityBadge from "@/components/SeverityBadge";
import { fmtTime, relativeTime } from "@/lib/time";
import { api } from "@/lib/api";
import { toast } from "@/components/Toast";
import { useHashState, useHashStateBool } from "@/hooks/useHashState";
import LiveBadge from "@/components/anim/LiveBadge";
import BlurText from "@/components/anim/BlurText";

interface Props {
  service: string;
  onServiceChange?: (s: string) => void;
}

type KindFilter = "log" | "metric" | "span";

export default function Live({ service, onServiceChange }: Props) {
  const events = useStream({ kinds: ["log", "metric", "span"], max: 500 });
  const [paused, setPaused] = useHashStateBool("paused", false);
  const [filterRaw, setFilterRaw] = useHashState("kind", "all");
  const filter: KindFilter | "all" =
    filterRaw === "log" || filterRaw === "metric" || filterRaw === "span"
      ? filterRaw
      : "all";
  const setFilter = (v: KindFilter | "all") => setFilterRaw(v);
  const [snap, setSnap] = useState<StreamEvent[]>([]);
  const tailRef = useRef<HTMLDivElement>(null);

  // Backfill on first mount.
  useEffect(() => {
    api.snapshot().then((s) => {
      const out: StreamEvent[] = [];
      for (const l of s.logs) {
        out.push({
          kind: "log",
          service: l.service,
          timestamp: new Date(l.timestamp).getTime(),
          body: l.body,
          status: l.severity,
          trace_id: l.trace_id,
          span_id: l.span_id,
        });
      }
      for (const m of s.metrics) {
        out.push({
          kind: "metric",
          service: m.service,
          timestamp: new Date(m.timestamp).getTime(),
          name: m.name,
          value: m.value,
        });
      }
      for (const sp of s.spans) {
        out.push({
          kind: "span",
          service: sp.service,
          timestamp: new Date(sp.start_time).getTime(),
          name: sp.name,
          trace_id: sp.trace_id,
          span_id: sp.span_id,
          status: sp.status,
        });
      }
      setSnap(out.sort((a, b) => b.timestamp - a.timestamp));
    });
  }, []);

  const all: StreamEvent[] = useMemo(() => {
    const map = new Map<string, StreamEvent>();
    for (const e of snap) map.set(`${e.kind}|${e.timestamp}|${e.service}|${e.body ?? e.name ?? ""}`, e);
    for (const e of events) {
      if (paused) continue;
      map.set(`${e.kind}|${e.timestamp}|${e.service}|${e.body ?? e.name ?? ""}`, e);
    }
    return Array.from(map.values()).sort((a, b) => b.timestamp - a.timestamp);
  }, [snap, events, paused]);

  const filtered = useMemo(
    () =>
      all.filter((e) => {
        if (service && e.service !== service) return false;
        if (filter !== "all" && e.kind !== filter) return false;
        return true;
      }),
    [all, filter, service]
  );

  // Counts.
  const counts = useMemo(() => {
    let logs = 0,
      metrics = 0,
      spans = 0;
    for (const e of filtered) {
      if (e.kind === "log") logs++;
      else if (e.kind === "metric") metrics++;
      else if (e.kind === "span") spans++;
    }
    return { logs, metrics, spans };
  }, [filtered]);

  // Auto-scroll on new events.
  useEffect(() => {
    if (!paused && tailRef.current) tailRef.current.scrollTop = 0;
  }, [events, paused]);

  const seed = async () => {
    try {
      const svc = service || "demo";
      const r = await api.seed(svc, 6);
      toast(`seeded ${r.seeded} events for ${r.service}`, "success");
    } catch (e) {
      toast(`seed failed: ${(e as Error).message}`, "error");
    }
  };

  return (
    <div className="p-4 space-y-3">
      <div className="bg-grafana-panel border border-grafana-border rounded-lg p-3 flex flex-wrap items-center gap-2 text-xs">
        <LiveBadge label="STREAMING" />
        <BlurText text="Live tail" duration={400} className="text-[13px] font-semibold mr-2" />
        <input
          value={service}
          onChange={(e) => onServiceChange?.(e.target.value)}
          placeholder="service (all)"
          className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 w-40 focus:outline-none focus:border-grafana-blue"
        />
        <div className="inline-flex bg-grafana-elev border border-grafana-border rounded">
          {(["all", "log", "metric", "span"] as const).map((k) => (
            <button
              key={k}
              onClick={() => setFilter(k)}
              className={`px-2.5 py-1 text-xs ${
                filter === k
                  ? "bg-grafana-accent/20 text-grafana-accent"
                  : "text-grafana-muted hover:text-grafana-text"
              }`}
            >
              {k}
            </button>
          ))}
        </div>
        <label className="text-grafana-muted flex items-center gap-1">
          <input
            type="checkbox"
            checked={paused}
            onChange={(e) => setPaused(e.target.checked)}
          />
          pause
        </label>
        <span className="ml-auto text-grafana-muted">
          <span className="text-grafana-ok">● live</span> ·{" "}
          <b className="text-grafana-text">{filtered.length}</b> shown
          {counts.metrics > 0 && <> · {counts.metrics} metric</>}
          {counts.spans > 0 && <> · {counts.spans} span</>}
        </span>
        <button
          onClick={seed}
          className="bg-grafana-blue/20 border border-grafana-blue/40 rounded px-3 py-1 text-grafana-blue"
        >
          + seed
        </button>
      </div>

      <div
        ref={tailRef}
        className="bg-grafana-panel border border-grafana-border rounded-lg font-mono text-[11px] overflow-y-auto scrollbar-thin"
        style={{ height: "calc(100vh - 180px)" }}
      >
        {filtered.length === 0 ? (
          <div className="text-grafana-muted italic text-center py-12">
            waiting for events… use <code className="text-grafana-accent">+ seed</code> or{" "}
            <code className="text-grafana-accent">/api/seed</code>.
          </div>
        ) : (
          <ul className="divide-y divide-grafana-border">
            {filtered.map((ev, i) => (
              <li
                key={i}
                className="px-3 py-1.5 hover:bg-grafana-elev/50 grid grid-cols-12 gap-2 items-center"
              >
                <span className="col-span-2 text-grafana-muted whitespace-nowrap">
                  <span title={fmtTime(ev.timestamp)}>{relativeTime(ev.timestamp)}</span>
                </span>
                <span
                  className={`col-span-1 text-[10px] uppercase tracking-wider ${
                    ev.kind === "log"
                      ? "text-grafana-accent"
                      : ev.kind === "metric"
                      ? "text-grafana-purple"
                      : "text-grafana-blue"
                  }`}
                >
                  {ev.kind}
                </span>
                <span className="col-span-2 text-grafana-text truncate">
                  {ev.service}
                </span>
                <span className="col-span-7 truncate">
                  {ev.kind === "log" && (
                    <>
                      {ev.status && <SeverityBadge value={ev.status as any} />}{" "}
                      <span title={ev.body}>{ev.body}</span>
                    </>
                  )}
                  {ev.kind === "metric" && (
                    <span className="text-grafana-purple">
                      {ev.name} = {ev.value?.toFixed(2)}
                    </span>
                  )}
                  {ev.kind === "span" && (
                    <span className="text-grafana-blue">
                      {ev.name}{" "}
                      <span className="text-grafana-muted">
                        trace={ev.trace_id?.slice(0, 8)}
                      </span>
                    </span>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
