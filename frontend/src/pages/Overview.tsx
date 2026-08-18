import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { ServiceSummary, HealthResponse } from "@/types/api";
import StatCard from "@/components/StatCard";
import TimeSeriesChart from "@/components/charts/TimeSeriesChart";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import { fmtTime, relativeTime } from "@/lib/time";
import FadeIn from "@/components/anim/FadeIn";
import BlurText from "@/components/anim/BlurText";

interface Props {
  onServiceSelect: (s: string) => void;
  onServiceDrillIn: (s: string) => void;
}

export default function Overview({ onServiceSelect, onServiceDrillIn }: Props) {
  const [services, setServices] = useState<ServiceSummary[]>([]);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [qpsSeries, setQpsSeries] = useState<Array<{ name: string; points: { ts: number; value: number }[]; color?: string }>>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [s, h, q] = await Promise.all([
          api.services(),
          api.health(),
          api.qps(5),
        ]);
        if (cancelled) return;
        setServices(s.services);
        setHealth(h);
        const palette = ["#3b82f6", "#10b981", "#a855f7", "#f59e0b", "#ef4444", "#06b6d4", "#84cc16"];
        setQpsSeries(
          q.series.map((row, i) => ({
            name: row.service,
            points: row.points,
            color: palette[i % palette.length],
          }))
        );
        setErr(null);
      } catch (e) {
        if (!cancelled) setErr(e as Error);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    const id = window.setInterval(load, 4000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  const totalLogs = services.reduce((s, x) => s + x.logs_count, 0);
  const totalMetrics = services.reduce((s, x) => s + x.metrics_count, 0);
  const totalSpans = services.reduce((s, x) => s + x.spans_count, 0);
  const avgError =
    services.length > 0
      ? services.reduce((s, x) => s + x.error_rate, 0) / services.length
      : 0;

  return (
    <FadeIn className="p-6 space-y-6">
      {err && <ErrorBox error={err} />}

      <div className="text-[13px] font-semibold mb-2">
        <BlurText text="Active fleet overview" duration={400} />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <StatCard
          title="Active services"
          value={services.length}
          hint={`uptime ${health?.uptime ?? "--"}`}
          accent="blue"
        />
        <StatCard
          title="Logs ingested"
          value={totalLogs.toLocaleString()}
          hint={`hot ${health?.engine.hot_logs ?? 0} · cold ${health?.engine.cold_logs ?? 0}`}
          accent="green"
        />
        <StatCard
          title="Metric points"
          value={totalMetrics.toLocaleString()}
          hint={`hot ${health?.engine.hot_metrics ?? 0} · cold ${health?.engine.cold_metrics ?? 0}`}
          accent="purple"
        />
        <StatCard
          title="Spans recorded"
          value={totalSpans.toLocaleString()}
          hint={`avg error ${(avgError * 100).toFixed(1)}%`}
          accent="amber"
          delta={services.length > 0 ? avgError * 100 - 1 : 0}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2 bg-grafana-panel border border-grafana-border rounded-lg p-4">
          <div className="flex items-center justify-between mb-2">
            <div>
              <div className="text-[13px] font-semibold">Request rate (QPS)</div>
              <div className="text-[11px] text-grafana-muted">
                Aggregated from hot metrics, last 5 minutes
              </div>
            </div>
            <button
              className="text-[11px] text-grafana-muted hover:text-grafana-text"
              onClick={() => window.location.reload()}
            >
              ⟳ refresh
            </button>
          </div>
          {loading ? (
            <Skeleton rows={1} />
          ) : qpsSeries.length === 0 ? (
            <div className="text-grafana-muted text-sm py-8 text-center">
              No data yet — try /api/seed or the Ingest demo.
            </div>
          ) : (
            <TimeSeriesChart series={qpsSeries} height={220} showLegend hideEmpty />
          )}
        </div>
        <div className="bg-grafana-panel border border-grafana-border rounded-lg p-4">
          <div className="text-[13px] font-semibold mb-2">Services</div>
          <div className="space-y-1.5 max-h-[220px] overflow-y-auto scrollbar-thin">
            {services.map((s) => {
              const err = s.error_rate * 100;
              const dot =
                err > 5
                  ? "bg-grafana-err"
                  : err > 1
                  ? "bg-grafana-warn"
                  : "bg-grafana-ok";
              return (
                <button
                  key={s.name}
                  onClick={() => onServiceDrillIn(s.name)}
                  className="w-full text-left px-2 py-1.5 rounded hover:bg-grafana-elev flex items-center justify-between text-sm transition-colors"
                >
                  <span className="flex items-center gap-2 truncate">
                    <span className={`w-1.5 h-1.5 rounded-full ${dot}`} />
                    <span className="truncate">{s.name}</span>
                  </span>
                  <span className="text-[10px] text-grafana-muted font-mono">
                    p99 {s.p99_ms.toFixed(0)}ms
                  </span>
                </button>
              );
            })}
            {services.length === 0 && (
              <div className="text-grafana-muted text-sm italic">
                No services yet. Use the Ingest demo or /api/seed.
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[13px] font-semibold flex items-center justify-between">
          <span>Service inventory</span>
          <span className="text-[11px] text-grafana-muted">
            {services.length} services · click row to drill in
          </span>
        </div>
        <table className="w-full text-sm">
          <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
            <tr>
              <th className="px-3 py-2 text-left">Service</th>
              <th className="px-3 py-2 text-right">Logs</th>
              <th className="px-3 py-2 text-right">Metrics</th>
              <th className="px-3 py-2 text-right">Spans</th>
              <th className="px-3 py-2 text-right">Err</th>
              <th className="px-3 py-2 text-right">p50</th>
              <th className="px-3 py-2 text-right">p95</th>
              <th className="px-3 py-2 text-right">p99</th>
              <th className="px-3 py-2 text-right">QPS</th>
              <th className="px-3 py-2 text-right">Updated</th>
            </tr>
          </thead>
          <tbody>
            {services.map((s) => (
              <tr
                key={s.name}
                onClick={() => onServiceDrillIn(s.name)}
                className="border-t border-grafana-border hover:bg-grafana-elev/50 cursor-pointer"
              >
                <td className="px-3 py-1.5 font-medium">{s.name}</td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {s.logs_count}
                </td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {s.metrics_count}
                </td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {s.spans_count}
                </td>
                <td className="px-3 py-1.5 text-right">
                  <span
                    className={
                      s.error_rate > 0.1
                        ? "text-grafana-err"
                        : s.error_rate > 0.05
                        ? "text-grafana-warn"
                        : "text-grafana-ok"
                    }
                  >
                    {(s.error_rate * 100).toFixed(1)}%
                  </span>
                </td>
                <td className="px-3 py-1.5 text-right font-mono text-grafana-muted">
                  {s.p50_ms.toFixed(0)}ms
                </td>
                <td className="px-3 py-1.5 text-right font-mono text-grafana-muted">
                  {s.p95_ms.toFixed(0)}ms
                </td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {s.p99_ms.toFixed(0)}ms
                </td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {s.qps.toFixed(2)}
                </td>
                <td className="px-3 py-1.5 text-right text-[11px] text-grafana-muted">
                  <span title={fmtTime(s.updated_at)}>
                    {relativeTime(s.updated_at)}
                  </span>
                </td>
              </tr>
            ))}
            {services.length === 0 && (
              <tr>
                <td colSpan={10} className="px-3 py-6 text-center text-grafana-muted italic">
                  No services yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </FadeIn>
  );
}
