import { useEffect, useState } from "react";
import { api, promMetrics } from "@/lib/api";
import type { DataSource } from "@/types/api";
import { fmtTime } from "@/lib/time";
import { ErrorBox } from "@/components/Feedback";

const HTTP_ENDPOINTS: Array<{ method: string; path: string; desc: string }> = [
  { method: "GET", path: "/api/health", desc: "engine health, hot/cold counters, uptime" },
  { method: "GET", path: "/api/services", desc: "list of services with p50/p95/p99 + QPS + err rate" },
  { method: "GET", path: "/api/services/{name}", desc: "single service summary" },
  { method: "GET", path: "/api/services/{name}/detail", desc: "drill-down: endpoints, recent errors, traces, QPS" },
  { method: "GET", path: "/api/query", desc: "unified query API for logs/metrics/traces" },
  { method: "GET", path: "/api/labels", desc: "known attribute keys for label-filter dropdown" },
  { method: "GET", path: "/api/service-map", desc: "caller → callee edges derived from spans" },
  { method: "GET", path: "/api/traces/{id}", desc: "all spans belonging to a trace" },
  { method: "GET", path: "/api/qps", desc: "per-service QPS time series (default 5m window)" },
  { method: "GET", path: "/api/histogram", desc: "log-binned latency histogram for a service" },
  { method: "GET", path: "/api/severity", desc: "count per log severity level" },
  { method: "GET", path: "/api/snapshot", desc: "recent records (used for live-tail backfill)" },
  { method: "GET", path: "/api/metric-names", desc: "top N metric names by ingest volume" },
  { method: "GET", path: "/api/dashboards", desc: "list of built-in dashboards" },
  { method: "GET", path: "/api/dashboards/{id}/panels", desc: "panel layout for a dashboard" },
  { method: "GET", path: "/api/export", desc: "export query results as CSV or JSON" },
  { method: "POST", path: "/api/ingest/otlp", desc: "OTLP simplified payload" },
  { method: "POST", path: "/api/ingest/otlp-json", desc: "OTLP/JSON standard envelope (resourceSpans/Logs/Metrics)" },
  { method: "GET", path: "/api/stream", desc: "WebSocket stream of accepted records" },
  { method: "GET", path: "/metrics", desc: "Prometheus exposition (dog_* counters/gauges)" },
];

export default function DataSources() {
  const [items, setItems] = useState<DataSource[]>([]);
  const [err, setErr] = useState<Error | null>(null);
  const [promPreview, setPromPreview] = useState<string>("");

  useEffect(() => {
    api.datasources().then((r) => setItems(r.datasources)).catch((e) => setErr(e));
  }, []);

  useEffect(() => {
    promMetrics().then(setPromPreview).catch(() => setPromPreview(""));
  }, []);

  return (
    <div className="p-4 space-y-4">
      {err && <ErrorBox error={err} />}
      <div>
        <div className="text-[13px] font-semibold">Data sources & API reference</div>
        <div className="text-[11px] text-grafana-muted">
          Pluggable OLAP backends and the wire-protocol contract for OTLP ingestion.
        </div>
      </div>

      <div className="space-y-3">
        {items.map((ds) => (
          <div
            key={ds.id}
            className="bg-grafana-panel border border-grafana-border rounded-lg p-4"
          >
            <div className="flex items-start gap-3">
              <div className="w-10 h-10 rounded bg-gradient-to-br from-grafana-accent to-grafana-accent2 flex items-center justify-center font-bold text-white text-sm">
                D
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <div className="text-[14px] font-semibold">{ds.name}</div>
                  <span className="text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-grafana-elev text-grafana-muted">
                    {ds.type}
                  </span>
                  {ds.default && (
                    <span className="text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-grafana-accent/20 text-grafana-accent">
                      default
                    </span>
                  )}
                </div>
                <div className="text-[11px] text-grafana-muted mt-1">{ds.description}</div>
                <div className="mt-3 grid grid-cols-2 md:grid-cols-4 gap-3 text-[11px]">
                  <div>
                    <div className="text-grafana-muted">URL</div>
                    <div className="font-mono text-grafana-text">{ds.url}</div>
                  </div>
                  <div>
                    <div className="text-grafana-muted">Database</div>
                    <div className="font-mono text-grafana-text">{ds.database}</div>
                  </div>
                  <div>
                    <div className="text-grafana-muted">Plugin version</div>
                    <div className="font-mono text-grafana-text">{ds.plugin_version}</div>
                  </div>
                  <div>
                    <div className="text-grafana-muted">Server version</div>
                    <div className="font-mono text-grafana-text">{ds.version}</div>
                  </div>
                </div>
                <div className="mt-3">
                  <div className="text-grafana-muted text-[11px] mb-1">Tables</div>
                  <div className="flex flex-wrap gap-1">
                    {ds.tables.map((t) => (
                      <span
                        key={t}
                        className="font-mono text-[11px] px-2 py-0.5 bg-grafana-elev border border-grafana-border rounded"
                      >
                        {t}
                      </span>
                    ))}
                  </div>
                </div>
                <div className="mt-3">
                  <div className="text-grafana-muted text-[11px] mb-1">Capabilities</div>
                  <div className="flex flex-wrap gap-1">
                    {ds.capabilities.map((c) => (
                      <span
                        key={c}
                        className="text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-grafana-accent2/20 text-grafana-accent2"
                      >
                        {c}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[13px] font-semibold">
          HTTP endpoints
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
              <tr>
                <th className="px-3 py-2 text-left w-16">Method</th>
                <th className="px-3 py-2 text-left">Path</th>
                <th className="px-3 py-2 text-left">Description</th>
              </tr>
            </thead>
            <tbody>
              {HTTP_ENDPOINTS.map((e) => (
                <tr key={e.method + e.path} className="border-t border-grafana-border">
                  <td className="px-3 py-1.5">
                    <span
                      className={
                        e.method === "GET"
                          ? "text-grafana-blue font-mono text-[11px]"
                          : "text-grafana-accent font-mono text-[11px]"
                      }
                    >
                      {e.method}
                    </span>
                  </td>
                  <td className="px-3 py-1.5 font-mono text-[12px]">{e.path}</td>
                  <td className="px-3 py-1.5 text-[12px] text-grafana-muted">{e.desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="bg-grafana-panel border border-grafana-border rounded-lg">
        <div className="px-4 py-2 border-b border-grafana-border text-[13px] font-semibold flex items-center justify-between">
          <span>/metrics (Prometheus exposition)</span>
          <a
            href="/metrics"
            target="_blank"
            rel="noreferrer"
            className="text-[11px] text-grafana-blue hover:underline"
          >
            Open raw ↗
          </a>
        </div>
        <pre className="px-4 py-3 font-mono text-[11px] text-grafana-text max-h-[260px] overflow-y-auto scrollbar-thin whitespace-pre-wrap">
{promPreview.slice(0, 4000) || "(loading…)"}
        </pre>
      </div>
    </div>
  );
}
