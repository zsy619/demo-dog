import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useStream } from "@/hooks/useStream";
import SeverityBadge from "@/components/SeverityBadge";
import { fmtTime } from "@/lib/time";
import { toast } from "@/components/Toast";
import { useHashState, useHashStateBool } from "@/hooks/useHashState";

const PRESETS = ["checkout", "search", "inventory", "auth", "recommend", "ads"];

const SAMPLE_OTLP = JSON.stringify(
  {
    resourceSpans: [
      {
        resource: { attributes: [{ key: "service.name", value: { stringValue: "checkout" } }] },
        scopeSpans: [
          {
            scope: { name: "demo" },
            spans: [
              {
                traceId: "00000000000000000000000000000001",
                spanId: "00000001",
                parentSpanId: "",
                name: "POST /checkout",
                kind: 1,
                startTimeUnixNano: String(Date.now() * 1_000_000),
                endTimeUnixNano: String((Date.now() + 50) * 1_000_000),
                attributes: [
                  { key: "http.method", value: { stringValue: "POST" } },
                  { key: "http.status_code", value: { intValue: "200" } },
                ],
                status: { code: 1, message: "" },
              },
            ],
          },
        ],
      },
    ],
    resourceLogs: [
      {
        resource: { attributes: [{ key: "service.name", value: { stringValue: "checkout" } }] },
        scopeLogs: [
          {
            logRecords: [
              {
                timeUnixNano: String(Date.now() * 1_000_000),
                severityNumber: 9,
                severityText: "INFO",
                body: { stringValue: "order placed" },
                attributes: [{ key: "order.id", value: { stringValue: "o-1" } }],
                traceId: "00000000000000000000000000000001",
                spanId: "00000001",
              },
            ],
          },
        ],
      },
    ],
    resourceMetrics: [
      {
        resource: { attributes: [{ key: "service.name", value: { stringValue: "checkout" } }] },
        scopeMetrics: [
          {
            metrics: [
              {
                name: "http.server.duration",
                unit: "ms",
                histogram: {
                  dataPoints: [
                    {
                      startTimeUnixNano: String(Date.now() * 1_000_000),
                      timeUnixNano: String(Date.now() * 1_000_000),
                      count: "1",
                      sum: 50,
                      attributes: [{ key: "http.method", value: { stringValue: "POST" } }],
                    },
                  ],
                },
              },
            ],
          },
        ],
      },
    ],
  },
  null,
  2
);

const SAMPLE_OTLP_SIMPLE = JSON.stringify(
  {
    resource_attrs: { "service.name": "checkout" },
    logs: [
      { timestamp: new Date().toISOString(), service: "checkout", severity: "INFO", body: "hello from demo payload" },
    ],
    metrics: [
      { timestamp: new Date().toISOString(), service: "checkout", name: "http.server.duration", value: 42, type: "gauge" },
    ],
    spans: [
      { trace_id: "00000000000000000000000000000002", span_id: "s2", name: "GET /demo", service: "checkout", start_time: new Date().toISOString(), duration_ms: 12, status: "ok" },
    ],
  },
  null,
  2
);

export default function IngestDemo() {
  // Service + count + mode live in the URL so the user can drop a colleague
  // a deep link like /#/ingest?service=auth&n=50&mode=otlp and have the
  // panel pre-populated.
  const [service, setService] = useHashState("service", "checkout");
  const [n, setN] = useHashState<number>("n", 10,
    (raw) => Math.max(1, Math.min(1000, parseInt(raw, 10) || 10)),
    (val) => String(val));
  const [busy, setBusy] = useState(false);
  const [lastResult, setLastResult] = useState<{
    ok: boolean;
    msg: string;
    body: Record<string, unknown> | null;
  } | null>(null);
  const [streamMode, setStreamMode] = useHashStateBool("stream", false);
  const [modeRaw, setModeRaw] = useHashState("mode", "simple");
  const mode: "simple" | "otlp" = modeRaw === "otlp" ? "otlp" : "simple";
  const setMode = (v: "simple" | "otlp") => setModeRaw(v);
  const [payload, setPayload] = useState(SAMPLE_OTLP_SIMPLE);
  const liveEvents = useStream({ kinds: ["log", "metric", "span"], max: 100 });

  // Track the most recent payloads the engine received. The backend returns
  // the raw OTLPRequest (Logs/Metrics/Spans/ResourceAttrs) so we derive a
  // summary on the fly.
  const [recent, setRecent] = useState<
    Array<{
      ts: number;
      kind: string;
      service: string;
      count: number;
    }>
  >([]);
  const refreshRecent = async () => {
    try {
      const r = await api.recentPayloads();
      const items = (r.payloads ?? []).slice(-12).reverse();
      const out: Array<{
        ts: number;
        kind: string;
        service: string;
        count: number;
      }> = [];
      for (const p of items) {
        const obj = p as Record<string, unknown>;
        const logs = (obj["Logs"] ?? obj["logs"]) as
          | Array<{ timestamp?: string | number }>
          | undefined;
        const metrics = (obj["Metrics"] ?? obj["metrics"]) as unknown[] | undefined;
        const spans = (obj["Spans"] ?? obj["spans"]) as unknown[] | undefined;
        const lastLogTs =
          logs && logs.length > 0 ? logs[logs.length - 1].timestamp : undefined;
        const ts = lastLogTs
          ? typeof lastLogTs === "number"
            ? lastLogTs > 1e12
              ? lastLogTs
              : lastLogTs * 1000
            : Date.parse(String(lastLogTs))
          : Date.now();
        const kinds: string[] = [];
        if (logs && logs.length > 0) kinds.push(`${logs.length} logs`);
        if (metrics && metrics.length > 0) kinds.push(`${metrics.length} metrics`);
        if (spans && spans.length > 0) kinds.push(`${spans.length} spans`);
        const kind = kinds.length > 0 ? kinds.join(" + ") : "—";
        const resourceAttrs =
          (obj["ResourceAttrs"] ?? obj["resource_attrs"]) as
            | Record<string, string>
            | undefined;
        const service = resourceAttrs?.["service.name"] ?? "—";
        const count =
          (logs?.length ?? 0) +
          (metrics?.length ?? 0) +
          (spans?.length ?? 0);
        out.push({ ts, kind, service, count });
      }
      setRecent(out);
    } catch {
      /* ignore */
    }
  };
  useEffect(() => {
    refreshRecent();
    const id = window.setInterval(refreshRecent, 5000);
    return () => window.clearInterval(id);
  }, []);

  const seed = async () => {
    setBusy(true);
    try {
      const r = await api.seed(service, n);
      setLastResult({
        ok: true,
        msg: `seeded ${r.seeded} records for service=${r.service} at ${new Date().toLocaleTimeString()}`,
        body: r as unknown as Record<string, unknown>,
      });
      toast(`seeded ${r.seeded} records`, "success");
      refreshRecent();
    } catch (e) {
      setLastResult({
        ok: false,
        msg: "seed error: " + (e as Error).message,
        body: null,
      });
      toast(`seed failed: ${(e as Error).message}`, "error");
    } finally {
      setBusy(false);
    }
  };

  const sendPayload = async () => {
    setBusy(true);
    try {
      const body = JSON.parse(payload);
      const resp = mode === "otlp" ? await api.ingestOTLPJSON(body) : await api.ingest(body);
      setLastResult({
        ok: true,
        msg: `ingested at ${new Date().toLocaleTimeString()}`,
        body: resp as Record<string, unknown>,
      });
      toast(`payload ingested`, "success");
      refreshRecent();
    } catch (e) {
      setLastResult({
        ok: false,
        msg: "ingest error: " + (e as Error).message,
        body: null,
      });
      toast(`ingest failed: ${(e as Error).message}`, "error");
    } finally {
      setBusy(false);
    }
  };

  const stream = async () => {
    setStreamMode(true);
    const es = new EventSource("/api/seed/stream");
    es.onmessage = (e) => {
      setLastResult({
        ok: true,
        msg: "stream → " + e.data,
        body: null,
      });
    };
    es.onerror = () => {
      setLastResult({ ok: false, msg: "stream closed", body: null });
      es.close();
      setStreamMode(false);
    };
    (window as unknown as { __es?: EventSource }).__es = es;
  };

  const stopStream = () => {
    const es = (window as unknown as { __es?: EventSource }).__es;
    if (es) {
      es.close();
      (window as unknown as { __es?: EventSource }).__es = undefined;
    }
    setStreamMode(false);
    setLastResult({ ok: true, msg: "stream stopped", body: null });
  };

  const switchMode = (m: "simple" | "otlp") => {
    setMode(m);
    setPayload(m === "otlp" ? SAMPLE_OTLP : SAMPLE_OTLP_SIMPLE);
  };

  return (
    <div className="p-4 grid grid-cols-12 gap-4">
      <div className="col-span-12 lg:col-span-7 bg-grafana-panel border border-grafana-border rounded-lg p-4 space-y-3">
        <div className="text-[13px] font-semibold">OTLP ingest demo</div>
        <div className="text-[11px] text-grafana-muted">
          Use the simulation endpoints to push synthetic Logs / Metrics / Traces
          into the in-memory Doris engine. The WebSocket live feed on the right
          will reflect every accepted record.
        </div>

        <div className="flex flex-wrap items-center gap-2 text-xs">
          <label className="text-grafana-muted">service</label>
          <input
            value={service}
            onChange={(e) => setService(e.target.value)}
            className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 text-grafana-text w-40"
            list="service-presets"
          />
          <datalist id="service-presets">
            {PRESETS.map((p) => (
              <option key={p} value={p} />
            ))}
          </datalist>
          <label className="text-grafana-muted">records</label>
          <input
            type="number"
            min={1}
            value={n}
            onChange={(e) => setN(Math.max(1, Number(e.target.value)))}
            className="w-20 bg-grafana-elev border border-grafana-border rounded px-2 py-1 text-grafana-text"
          />
          <button
            onClick={seed}
            disabled={busy}
            className="bg-grafana-accent text-white px-3 py-1 rounded hover:bg-grafana-accent/80 disabled:opacity-50"
          >
            {busy ? "Working…" : "Seed once"}
          </button>
          {!streamMode ? (
            <button
              onClick={stream}
              className="bg-grafana-elev border border-grafana-border px-3 py-1 rounded hover:bg-grafana-panel"
            >
              Start SSE stream
            </button>
          ) : (
            <button
              onClick={stopStream}
              className="bg-grafana-err/20 border border-grafana-err text-grafana-err px-3 py-1 rounded"
            >
              Stop SSE
            </button>
          )}
        </div>

        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs">
            <span className="text-grafana-muted">payload</span>
            <div className="inline-flex bg-grafana-elev border border-grafana-border rounded">
              <button
                onClick={() => switchMode("simple")}
                className={`px-2.5 py-1 text-xs ${mode === "simple" ? "bg-grafana-accent/20 text-grafana-accent" : "text-grafana-muted"}`}
              >
                simple JSON
              </button>
              <button
                onClick={() => switchMode("otlp")}
                className={`px-2.5 py-1 text-xs ${mode === "otlp" ? "bg-grafana-accent/20 text-grafana-accent" : "text-grafana-muted"}`}
              >
                OTLP/JSON
              </button>
            </div>
            <button
              onClick={sendPayload}
              disabled={busy}
              className="bg-grafana-blue/20 border border-grafana-blue/40 text-grafana-blue px-3 py-1 rounded hover:bg-grafana-blue/30 disabled:opacity-50"
            >
              Send payload
            </button>
            <button
              onClick={() => setPayload(mode === "otlp" ? SAMPLE_OTLP : SAMPLE_OTLP_SIMPLE)}
              className="text-grafana-muted text-[10px] hover:text-grafana-text"
            >
              reset
            </button>
          </div>
          <textarea
            value={payload}
            onChange={(e) => setPayload(e.target.value)}
            rows={10}
            className="w-full font-mono text-[11px] bg-grafana-elev border border-grafana-border rounded p-2"
          />
        </div>

        <div className="bg-grafana-elev border border-grafana-border rounded p-3 font-mono text-[11px]">
          <div className="text-grafana-muted"># synchronous seed</div>
          <div>curl -s "http://localhost:18080/api/seed?service={service}&n={n}"</div>
          <div className="text-grafana-muted mt-2"># SSE stream</div>
          <div>curl -N "http://localhost:18080/api/seed/stream"</div>
          <div className="text-grafana-muted mt-2"># raw OTLP write</div>
          <div>curl -X POST http://localhost:18080/api/ingest/otlp @-</div>
          <div className="text-grafana-muted mt-2"># Prometheus scrape</div>
          <div>curl http://localhost:18080/metrics</div>
        </div>

        {lastResult && (
          <div
            className={`text-[11px] font-mono bg-grafana-elev border rounded px-2 py-1.5 ${
              lastResult.ok
                ? "border-grafana-ok/30 text-grafana-text"
                : "border-grafana-err/40 text-grafana-err"
            }`}
          >
            <div className="flex items-center gap-1.5">
              <span
                className={`inline-block w-1.5 h-1.5 rounded-full ${
                  lastResult.ok ? "bg-grafana-ok" : "bg-grafana-err"
                }`}
              />
              <span>{lastResult.msg}</span>
            </div>
            {lastResult.body && (
              <details className="mt-1">
                <summary className="cursor-pointer text-grafana-muted hover:text-grafana-text">
                  response body
                </summary>
                <pre className="mt-1 text-[10px] text-grafana-muted whitespace-pre-wrap break-all max-h-32 overflow-y-auto scrollbar-thin">
                  {JSON.stringify(lastResult.body, null, 2)}
                </pre>
              </details>
            )}
          </div>
        )}
      </div>

      <div className="col-span-12 lg:col-span-5 bg-grafana-panel border border-grafana-border rounded-lg p-4">
        <div className="text-[11px] uppercase tracking-wider text-grafana-muted mb-2 flex justify-between">
          <span>WebSocket live feed</span>
          <span className="text-grafana-ok flex items-center gap-1">
            <span className="inline-block w-2 h-2 rounded-full bg-grafana-ok animate-pulse-soft" />
            ws
          </span>
        </div>
        <div className="space-y-1 max-h-[560px] overflow-y-auto scrollbar-thin font-mono text-[11px]">
          {liveEvents.length === 0 && (
            <div className="text-grafana-muted italic">no events yet</div>
          )}
          {liveEvents.map((ev, i) => (
            <div key={i} className="flex items-start gap-2 border-b border-grafana-border pb-1">
              <span className="text-grafana-muted whitespace-nowrap w-16 shrink-0">
                {fmtTime(ev.timestamp)}
              </span>
              <span
                className={`whitespace-nowrap ${
                  ev.kind === "log"
                    ? "text-grafana-ok"
                    : ev.kind === "metric"
                    ? "text-grafana-accent"
                    : "text-grafana-accent2"
                }`}
              >
                [{ev.kind}]
              </span>
              {ev.kind === "log" && <SeverityBadge value={(ev.status as any) ?? "INFO"} />}
              <span className="text-grafana-text whitespace-nowrap">{ev.service}</span>
              <span className="truncate" title={ev.body ?? ev.name ?? ""}>
                {ev.body ?? `${ev.name}=${ev.value ?? ""}`}
              </span>
            </div>
          ))}
        </div>
      </div>

      <div className="col-span-12 bg-grafana-panel border border-grafana-border rounded-lg p-4">
        <div className="flex items-center justify-between mb-2">
          <div className="text-[13px] font-semibold">Recent ingest payloads</div>
          <button
            onClick={refreshRecent}
            className="text-[11px] text-grafana-muted hover:text-grafana-text"
          >
            refresh
          </button>
        </div>
        {recent.length === 0 ? (
          <div className="text-grafana-muted italic text-xs py-6 text-center">
            no payloads yet — try a seed or POST your own OTLP JSON.
          </div>
        ) : (
          <table className="w-full text-xs">
            <thead className="bg-grafana-elev text-[10px] text-grafana-muted uppercase tracking-wider">
              <tr>
                <th className="px-3 py-1.5 text-left">When</th>
                <th className="px-3 py-1.5 text-left">Service</th>
                <th className="px-3 py-1.5 text-left">Kind</th>
                <th className="px-3 py-1.5 text-right">Records</th>
              </tr>
            </thead>
            <tbody>
              {recent.map((r, i) => (
                <tr key={i} className="border-t border-grafana-border">
                  <td className="px-3 py-1 text-grafana-muted">
                    {new Date(r.ts * (r.ts > 1e12 ? 1 : 1000)).toLocaleTimeString()}
                  </td>
                  <td className="px-3 py-1 text-grafana-accent font-mono">
                    {r.service}
                  </td>
                  <td className="px-3 py-1 text-grafana-text">{r.kind}</td>
                  <td className="px-3 py-1 text-right font-mono tabular-nums">
                    {r.count}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
