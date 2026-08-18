import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { ServiceDetail, LogRecord, EndpointStats } from "@/types/api";
import TimeSeriesChart from "@/components/charts/TimeSeriesChart";
import StatCard from "@/components/StatCard";
import SeverityBadge from "@/components/SeverityBadge";
import Highlight from "@/components/Highlight";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import { fmtTime, duration, relativeTime } from "@/lib/time";
import { toast } from "@/components/Toast";
import Magnetic from "@/components/anim/Magnetic";
import GradientText from "@/components/anim/GradientText";
import BlurText from "@/components/anim/BlurText";

interface Props {
  service: string;
  onNavigate: (page: string, params: Record<string, string>) => void;
}

export default function ServiceDetailPage({ service, onNavigate }: Props) {
  const [data, setData] = useState<ServiceDetail | null>(null);
  const [recentLogs, setRecentLogs] = useState<LogRecord[]>([]);
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    // The Service Detail page only makes sense once a service has been picked.
    // Without this guard the user lands here with an empty `service` prop
    // (e.g. via the Sidebar header click) and the page would fire
    // `GET /api/services//detail`, which the backend rightly rejects with 404.
    if (!service) {
      setData(null);
      setRecentLogs([]);
      setLoading(false);
      setErr(null);
      return;
    }
    setLoading(true);
    const load = async () => {
      try {
        const d = await api.serviceDetail(service);
        if (!cancelled) {
          setData(d);
          setErr(null);
        }
      } catch (e) {
        if (!cancelled) setErr(e as Error);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    const loadLogs = async () => {
      try {
        const r = await api.query({
          type: "logs",
          service,
          limit: 50,
        });
        if (!cancelled) setRecentLogs((r.rows as unknown as LogRecord[]) ?? []);
      } catch {
        // ignore
      }
    };
    load();
    loadLogs();
    const id1 = window.setInterval(load, 6000);
    const id2 = window.setInterval(loadLogs, 6000);
    return () => {
      cancelled = true;
      window.clearInterval(id1);
      window.clearInterval(id2);
    };
  }, [service]);

  if (loading && !data) {
    return <Skeleton rows={5} />;
  }
  if (err) {
    return <ErrorBox error={err} />;
  }
  // Friendly empty state when the user navigates here without picking a
  // service (e.g. the Sidebar header clicks "Service" with nothing selected).
  if (!service) {
    return (
      <div className="p-8 text-center text-grafana-muted space-y-2">
        <div className="text-[14px] font-semibold text-grafana-text">
          No service selected
        </div>
        <div className="text-[12px]">
          Pick a service from the sidebar to drill into its logs, latency and
          endpoints.
        </div>
      </div>
    );
  }
  if (!data) {
    return (
      <div className="text-grafana-muted italic p-4">
        No data for service <code className="text-grafana-accent">{service}</code>.
      </div>
    );
  }

  const { summary } = data;
  // The backend occasionally serializes empty slices as `null` (a Go quirk
  // for nil slices). Coerce every list field to a real array up-front so
  // downstream `.length` reads and `.map(...)` calls never crash with
  // "Cannot read properties of null".
  const qps = data.qps ?? [];
  const endpoints = data.endpoints ?? [];
  const topOps = data.top_ops ?? [];
  const recentErrors = data.recent_errors ?? [];
  const recentTraces = data.recent_traces ?? [];
  const metricNames = data.metric_names ?? [];
  const errColor =
    summary.error_rate > 0.05 ? "err" : summary.error_rate > 0.01 ? "warn" : "green";

  return (
    <div className="p-4 space-y-4">
      {/* Breadcrumbs */}
      <div className="flex items-center gap-1 text-[11px] text-grafana-muted">
        <button
          className="hover:text-grafana-text"
          onClick={() => onNavigate("overview", {})}
        >
          overview
        </button>
        <span>›</span>
        <button
          className="hover:text-grafana-text"
          onClick={() => onNavigate("service-map", {})}
        >
          services
        </button>
        <span>›</span>
        <span className="text-grafana-text font-medium">{summary.name}</span>
      </div>

      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-2">
        <div>
          <div className="text-[11px] text-grafana-muted uppercase tracking-wider">
            Service
          </div>
          <GradientText className="text-2xl font-semibold">
            <BlurText text={summary.name} duration={500} />
          </GradientText>
          <div className="text-[11px] text-grafana-muted">
            last seen {relativeTime(new Date(summary.updated_at).getTime())}
          </div>
        </div>
        <div className="flex items-center gap-2 text-xs">
          <button
            className="bg-grafana-elev border border-grafana-border rounded px-3 py-1 hover:bg-grafana-elev2"
            onClick={() => onNavigate("logs", { service: summary.name })}
          >
            Open logs
          </button>
          <button
            className="bg-grafana-elev border border-grafana-border rounded px-3 py-1 hover:bg-grafana-elev2"
            onClick={() => onNavigate("traces", { service: summary.name })}
          >
            Open traces
          </button>
          <button
            className="bg-grafana-accent text-white px-3 py-1 rounded hover:bg-grafana-accent/80"
            onClick={() => onNavigate("explore", { service: summary.name, signal: "metrics" })}
          >
            Metrics
          </button>
        </div>
      </div>

      {/* KPI cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
        <Magnetic strength={4}><StatCard title="QPS" value={summary.qps.toFixed(2)} accent="blue" /></Magnetic>
        <Magnetic strength={4}><StatCard title="p50" value={`${summary.p50_ms.toFixed(0)}ms`} accent="blue" /></Magnetic>
        <Magnetic strength={4}><StatCard title="p95" value={`${summary.p95_ms.toFixed(0)}ms`} accent="purple" /></Magnetic>
        <Magnetic strength={4}><StatCard title="p99" value={`${summary.p99_ms.toFixed(0)}ms`} accent="purple" /></Magnetic>
        <Magnetic strength={4}><StatCard
          title="Error rate"
          value={`${(summary.error_rate * 100).toFixed(2)}%`}
          accent={errColor}
        /></Magnetic>
      </div>

      {/* QPS chart */}
      <div className="bg-grafana-panel border border-grafana-border rounded-lg p-3">
        <div className="flex items-center justify-between mb-2">
          <div className="text-[13px] font-semibold">QPS (last 5m)</div>
          <div className="text-[11px] text-grafana-muted">
            {qps.length} points
          </div>
        </div>
        {qps.length === 0 ? (
          <Skeleton rows={1} />
        ) : (
          <TimeSeriesChart
            series={[
              {
                name: summary.name,
                points: qps,
                color: "#3b82f6",
              },
            ]}
            height={160}
          />
        )}
      </div>

      <div className="grid grid-cols-12 gap-3">
        {/* Endpoints / span names */}
        <div className="col-span-12 lg:col-span-7 bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
          <div className="px-4 py-2 border-b border-grafana-border text-[12px] flex items-center justify-between">
            <span className="text-grafana-muted">Top endpoints</span>
            <span className="text-grafana-muted">{endpoints.length}</span>
          </div>
          {endpoints.length === 0 ? (
            <div className="px-4 py-6 text-grafana-muted italic">No spans yet.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
                <tr>
                  <th className="px-3 py-2 text-left">Endpoint</th>
                  <th className="px-3 py-2 text-right">Calls</th>
                  <th className="px-3 py-2 text-right">Err</th>
                  <th className="px-3 py-2 text-right">Err %</th>
                  <th className="px-3 py-2 text-right">Avg</th>
                  <th className="px-3 py-2 text-right">p99</th>
                </tr>
              </thead>
              <tbody>
                {endpoints.map((e) => (
                  <tr
                    key={e.name}
                    className="border-t border-grafana-border hover:bg-grafana-elev/40 cursor-pointer"
                    onClick={() => {
                      onNavigate("explore", {
                        service: summary.name,
                        signal: "traces",
                        search: e.name,
                      });
                    }}
                    title={`Click to filter traces by ${e.name}`}
                  >
                    <td className="px-3 py-1.5 font-mono text-[12px]">{e.name}</td>
                    <td className="px-3 py-1.5 text-right font-mono">{e.count}</td>
                    <td className="px-3 py-1.5 text-right font-mono">
                      <span className={e.errors > 0 ? "text-grafana-err" : ""}>
                        {e.errors}
                      </span>
                    </td>
                    <td className="px-3 py-1.5 text-right">
                      <span
                        className={
                          e.errors / Math.max(1, e.count) > 0.05
                            ? "text-grafana-err"
                            : "text-grafana-muted"
                        }
                      >
                        {((e.errors / Math.max(1, e.count)) * 100).toFixed(1)}%
                      </span>
                    </td>
                    <td className="px-3 py-1.5 text-right font-mono">
                      {duration(e.avg_ms)}
                    </td>
                    <td className="px-3 py-1.5 text-right font-mono">
                      {duration(e.p99_ms)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Metric names */}
        <div className="col-span-12 lg:col-span-5 bg-grafana-panel border border-grafana-border rounded-lg p-3">
          <div className="text-[12px] text-grafana-muted mb-2">Emitted metrics</div>
          {metricNames.length === 0 ? (
            <div className="text-grafana-muted italic">No metric points yet.</div>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {metricNames.map((n) => (
                <button
                  key={n}
                  className="text-[11px] font-mono bg-grafana-elev border border-grafana-border rounded px-2 py-1 hover:bg-grafana-elev2 hover:border-grafana-accent"
                  onClick={() =>
                    onNavigate("explore", {
                      service: summary.name,
                      signal: "metrics",
                      metric: n,
                    })
                  }
                >
                  {n}
                </button>
              ))}
            </div>
          )}
          <div className="mt-3 text-[12px] text-grafana-muted mb-1">Recent traces</div>
          {recentTraces.length === 0 ? (
            <div className="text-grafana-muted italic text-xs">no traces</div>
          ) : (
            <div className="space-y-1 max-h-[220px] overflow-y-auto scrollbar-thin">
              {recentTraces.map((tid) => (
                <button
                  key={tid}
                  className="w-full text-left text-[11px] font-mono bg-grafana-elev border border-grafana-border rounded px-2 py-1 hover:bg-grafana-elev2"
                  onClick={() => {
                    onNavigate("traces", { trace_id: tid });
                    toast(`opening trace ${tid.slice(0, 8)}`, "success");
                  }}
                >
                  {tid}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Recent errors */}
      <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[12px] flex items-center justify-between">
          <span className="text-grafana-muted">Recent errors</span>
          <span className="text-grafana-muted">{recentErrors.length}</span>
        </div>
        {recentErrors.length === 0 ? (
          <div className="px-4 py-6 text-grafana-muted italic">
            No errors observed. ✨
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
              <tr>
                <th className="px-3 py-2 text-left">Time</th>
                <th className="px-3 py-2 text-left">Level</th>
                <th className="px-3 py-2 text-left">Message</th>
                <th className="px-3 py-2 text-left">Trace</th>
                <th className="px-3 py-2 text-left">Labels</th>
              </tr>
            </thead>
            <tbody>
              {recentErrors.map((r, i) => (
                <tr key={i} className="border-t border-grafana-border">
                  <td className="px-3 py-1.5 text-[11px] text-grafana-muted whitespace-nowrap">
                    {fmtTime(new Date(r.timestamp).getTime())}
                  </td>
                  <td className="px-3 py-1.5">
                    <SeverityBadge value={r.severity} />
                  </td>
                  <td className="px-3 py-1.5">
                    <Highlight text={r.body} query="" />
                  </td>
                  <td className="px-3 py-1.5">
                    {r.trace_id ? (
                      <button
                        className="text-grafana-blue font-mono text-[11px] hover:underline"
                        onClick={() => onNavigate("traces", { trace_id: r.trace_id! })}
                      >
                        {r.trace_id.slice(0, 12)}
                      </button>
                    ) : (
                      <span className="text-grafana-muted">—</span>
                    )}
                  </td>
                  <td className="px-3 py-1.5 text-[11px] font-mono text-grafana-muted">
                    {r.attributes
                      ? Object.entries(r.attributes)
                          .slice(0, 3)
                          .map(([k, v]) => `${k}=${v}`)
                          .join(" ")
                      : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Counts summary */}
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <StatCard
          title="Logs"
          value={summary.logs_count.toLocaleString()}
          accent="green"
        />
        <StatCard
          title="Metrics"
          value={summary.metrics_count.toLocaleString()}
          accent="purple"
        />
        <StatCard
          title="Spans"
          value={summary.spans_count.toLocaleString()}
          accent="blue"
        />
      </div>

      {/* Recent logs */}
      <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[12px] flex items-center justify-between">
          <span className="text-grafana-muted">Recent logs (last {recentLogs.length})</span>
          <button
            onClick={() => onNavigate("logs", { service: summary.name })}
            className="text-[11px] text-grafana-blue hover:underline"
          >
            open in logs ↗
          </button>
        </div>
        {recentLogs.length === 0 ? (
          <div className="px-4 py-6 text-grafana-muted italic">No logs yet.</div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
              <tr>
                <th className="px-3 py-2 text-left">Time</th>
                <th className="px-3 py-2 text-left">Level</th>
                <th className="px-3 py-2 text-left">Message</th>
                <th className="px-3 py-2 text-left">Trace</th>
              </tr>
            </thead>
            <tbody>
              {recentLogs.slice(0, 12).map((r, i) => (
                <tr key={i} className="border-t border-grafana-border">
                  <td className="px-3 py-1.5 text-[11px] text-grafana-muted whitespace-nowrap">
                    {fmtTime(new Date(r.timestamp).getTime())}
                  </td>
                  <td className="px-3 py-1.5">
                    <SeverityBadge value={r.severity} />
                  </td>
                  <td className="px-3 py-1.5 font-mono text-[12px]">
                    <Highlight text={r.body ?? ""} query="" />
                  </td>
                  <td className="px-3 py-1.5 text-[11px]">
                    {r.trace_id ? (
                      <button
                        className="text-grafana-blue font-mono hover:underline"
                        onClick={() => onNavigate("traces", { trace_id: r.trace_id! })}
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
  );
}
