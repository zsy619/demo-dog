import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { Dashboard, Panel, QueryResult, ServiceSummary } from "@/types/api";
import TimeSeriesChart from "@/components/charts/TimeSeriesChart";
import StatCard from "@/components/StatCard";
import Table from "@/components/Table";
import SeverityBadge from "@/components/SeverityBadge";
import TraceWaterfall from "@/components/charts/TraceWaterfall";
import { ErrorBox, Skeleton } from "@/components/Feedback";

function readDashboardFromUrl(): string {
  if (typeof window === "undefined") return "overview";
  const h = window.location.hash;
  const qIdx = h.indexOf("?");
  if (qIdx < 0) return "overview";
  const params = new URLSearchParams(h.slice(qIdx + 1));
  return params.get("dashboard") ?? "overview";
}


interface Props {
  onOpen: (id: string) => void;
}

export default function Dashboards({ onOpen }: Props) {
  const [items, setItems] = useState<Dashboard[]>([]);
  const [active, setActive] = useState<string>(readDashboardFromUrl());
  const [panels, setPanels] = useState<Panel[]>([]);
  const [services, setServices] = useState<ServiceSummary[]>([]);
  const [err, setErr] = useState<Error | null>(null);

  // Sync the URL hash whenever the user picks a different dashboard from the
  // sidebar, so the "Open in app" button and reloads land on the right page.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const h = window.location.hash;
    if (h.startsWith("#/dashboards")) {
      const qIdx = h.indexOf("?");
      const base = qIdx >= 0 ? h.slice(0, qIdx) : h;
      const params = new URLSearchParams(qIdx >= 0 ? h.slice(qIdx + 1) : "");
      if (params.get("dashboard") !== active) {
        params.set("dashboard", active);
        const next = `${base}?${params.toString()}`;
        window.history.replaceState(null, "", next);
      }
    }
  }, [active]);

  useEffect(() => {
    api.dashboards().then((r) => setItems(r.dashboards)).catch(() => {});
  }, []);

  useEffect(() => {
    if (!active) return;
    api.panels(active).then((r) => setPanels(r.panels)).catch((e) => setErr(e));
  }, [active]);

  // Listen for hash changes (e.g. the "Open in app" button on the same page
  // writes a new ?dashboard= value into the URL without re-mounting this
  // component; we need to pick that up).
  useEffect(() => {
    const onHash = () => {
      const next = readDashboardFromUrl();
      setActive((cur) => (cur === next ? cur : next));
    };
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  useEffect(() => {
    api.services().then((r) => setServices(r.services)).catch(() => {});
  }, []);

  // Aggregate top metrics for the dashboard "overview" panel.
  const overviewMetric = useMemo(() => {
    return services.map((s) => ({
      name: s.name,
      qps: s.qps,
      p99: s.p99_ms,
      err: s.error_rate,
    }));
  }, [services]);

  return (
    <div className="p-4 grid grid-cols-12 gap-4">
      {err && <ErrorBox error={err} className="col-span-12" />}
      <div className="col-span-12 lg:col-span-3">
        <div className="bg-grafana-panel border border-grafana-border rounded-lg">
          <div className="px-4 py-2 border-b border-grafana-border text-[13px] font-semibold">
            Built-in dashboards
          </div>
          <div className="divide-y divide-grafana-border">
            {items.map((d) => (
              <button
                key={d.id}
                onClick={() => setActive(d.id)}
                className={
                  "w-full text-left px-4 py-3 text-sm hover:bg-grafana-elev transition-colors " +
                  (active === d.id ? "bg-grafana-elev" : "")
                }
              >
                <div className="font-medium">{d.name}</div>
                <div className="text-[11px] text-grafana-muted mt-0.5">
                  {d.description}
                </div>
                <div className="flex gap-1 mt-1.5">
                  {d.tags.map((t) => (
                    <span
                      key={t}
                      className="text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-grafana-elev text-grafana-muted"
                    >
                      {t}
                    </span>
                  ))}
                </div>
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="col-span-12 lg:col-span-9 space-y-3">
        <div className="bg-grafana-panel border border-grafana-border rounded-lg p-4">
          <div className="flex items-center justify-between mb-3">
            <div className="text-[13px] font-semibold">
              {items.find((d) => d.id === active)?.name ?? "Overview"}
            </div>
            <button
              onClick={() => onOpen(active)}
              className="bg-grafana-accent text-white text-xs px-3 py-1 rounded hover:bg-grafana-accent/80"
            >
              Open in app
            </button>
          </div>

          {/* Special case: overview dashboard renders KPI cards + per-service bars */}
          {active === "overview" ? (
            <OverviewDashboard services={services} />
          ) : (
            <div className="grid grid-cols-12 gap-2">
              {panels.map((p) => (
                <PanelRenderer key={p.id} panel={p} />
              ))}
              {panels.length === 0 && (
                <div className="text-grafana-muted text-sm italic">
                  no panels
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function OverviewDashboard({ services }: { services: ServiceSummary[] }) {
  const [series, setSeries] = useState<Array<{ name: string; points: { ts: number; value: number }[]; color?: string }>>([]);
  useEffect(() => {
    const id = window.setInterval(() => {
      api.qps(5).then((r) => {
        const palette = ["#3b82f6", "#10b981", "#a855f7", "#f59e0b", "#ef4444", "#06b6d4"];
        setSeries(
          r.series.map((row, i) => ({
            name: row.service,
            points: row.points,
            color: palette[i % palette.length],
          }))
        );
      }).catch(() => {});
    }, 5000);
    return () => window.clearInterval(id);
  }, []);

  const totalLogs = services.reduce((s, x) => s + x.logs_count, 0);
  const totalSpans = services.reduce((s, x) => s + x.spans_count, 0);
  const avgErr =
    services.length > 0
      ? services.reduce((s, x) => s + x.error_rate, 0) / services.length
      : 0;

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatCard title="Services" value={services.length} accent="blue" />
        <StatCard title="Logs ingested" value={totalLogs.toLocaleString()} accent="green" />
        <StatCard title="Spans recorded" value={totalSpans.toLocaleString()} accent="purple" />
        <StatCard
          title="Avg error rate"
          value={`${(avgErr * 100).toFixed(2)}%`}
          accent={avgErr > 0.05 ? "amber" : "green"}
        />
      </div>
      <div className="bg-grafana-elev border border-grafana-border rounded p-3">
        <div className="text-[13px] font-semibold mb-2">Request rate (last 5m)</div>
        {series.length === 0 ? (
          <Skeleton rows={1} />
        ) : (
          <TimeSeriesChart series={series} height={220} />
        )}
      </div>
      <div className="bg-grafana-elev border border-grafana-border rounded overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[12px] text-grafana-muted">
          Per-service
        </div>
        <table className="w-full text-sm">
          <thead className="bg-grafana-panel text-[11px] text-grafana-muted uppercase tracking-wider">
            <tr>
              <th className="px-3 py-2 text-left">Service</th>
              <th className="px-3 py-2 text-right">QPS</th>
              <th className="px-3 py-2 text-right">p99</th>
              <th className="px-3 py-2 text-right">Errors</th>
              <th className="px-3 py-2 text-right">Logs</th>
            </tr>
          </thead>
          <tbody>
            {services.map((s) => (
              <tr key={s.name} className="border-t border-grafana-border">
                <td className="px-3 py-1.5">{s.name}</td>
                <td className="px-3 py-1.5 text-right font-mono">{s.qps.toFixed(2)}</td>
                <td className="px-3 py-1.5 text-right font-mono">{s.p99_ms.toFixed(0)}ms</td>
                <td className="px-3 py-1.5 text-right">
                  <span
                    className={
                      s.error_rate > 0.05
                        ? "text-grafana-err"
                        : s.error_rate > 0.01
                        ? "text-grafana-warn"
                        : "text-grafana-ok"
                    }
                  >
                    {(s.error_rate * 100).toFixed(2)}%
                  </span>
                </td>
                <td className="px-3 py-1.5 text-right font-mono">{s.logs_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function PanelRenderer({ panel }: { panel: Panel }) {
  const [data, setData] = useState<QueryResult | null>(null);
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const run = async () => {
      try {
        const type: "logs" | "metrics" | "traces" =
          panel.type === "logs"
            ? "logs"
            : panel.type === "metrics"
            ? "metrics"
            : panel.type === "traces"
            ? "traces"
            : panel.type === "stat"
            ? "metrics"
            : "logs";
        const res = await api.query({
          type,
          service: panel.config?.service,
          name: panel.config?.metric,
          window: (panel.config?.window as any) ?? "1m",
          trace_id: panel.config?.trace_id,
          severity: panel.config?.severity,
          limit: panel.config?.max_rows ?? 200,
        });
        if (!cancelled) setData(res);
      } catch (e) {
        if (!cancelled) setErr(e as Error);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    run();
    const id = window.setInterval(run, 6000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [panel.id, panel.config?.service, panel.config?.metric]);

  // Tailwind needs static class names; map the grid width to a literal set.
  const span = panel.grid?.w ?? 6;
  const spanCls =
    span <= 3
      ? "md:col-span-3"
      : span <= 4
      ? "md:col-span-4"
      : span <= 6
      ? "md:col-span-6"
      : span <= 8
      ? "md:col-span-8"
      : "md:col-span-12";

  return (
    <div
      className={`col-span-12 ${spanCls} bg-grafana-elev border border-grafana-border rounded p-3 min-h-[180px]`}
    >
      <div className="text-[11px] text-grafana-muted uppercase tracking-wider">
        {panel.type}
      </div>
      <div className="text-[13px] font-semibold mt-0.5">{panel.title}</div>

      {loading ? (
        <div className="mt-2">
          <Skeleton rows={2} />
        </div>
      ) : err ? (
        <ErrorBox error={err} className="mt-2" />
      ) : panel.type === "timeseries" && data?.series ? (
        <div className="mt-2">
          <TimeSeriesChart
            series={data.series.map((s, i) => ({
              name: s.name,
              points: s.points,
              color: ["#3b82f6", "#10b981", "#a855f7", "#f59e0b"][i % 4],
            }))}
            height={140}
          />
        </div>
      ) : panel.type === "stat" && data ? (
        <div className="text-2xl font-semibold mt-2 tabular-nums">
          {(() => {
            // Metric queries return series with `service`+`points`; log queries
            // return rows. We pick whichever shape actually has data.
            if (data.series && data.series.length > 0) {
              const pts = data.series[0].points;
              const last = pts[pts.length - 1];
              return last ? last.value.toFixed(2) : "0";
            }
            if (data.rows && data.rows.length > 0) {
              // For stat panels over logs (e.g. error count) we render the
              // total count rather than the value field, since `value` may
              // not exist on log rows.
              return data.rows.length.toString();
            }
            return "0";
          })()}
        </div>
      ) : panel.type === "logs" && data ? (
        <div className="mt-2 max-h-[200px] overflow-y-auto scrollbar-thin">
          <Table
            rows={(data.rows ?? []).slice(0, 30)}
            columns={[
              { key: "timestamp", label: "Time", width: "150px", mono: true },
              { key: "service", label: "Svc", width: "80px" },
              {
                key: "severity",
                label: "Lvl",
                width: "60px",
                render: (r) => <SeverityBadge value={String(r.severity ?? "INFO")} />,
              },
              { key: "body", label: "Message" },
            ]}
            emptyHint="no logs"
          />
        </div>
      ) : panel.type === "traces" && data ? (
        <div className="mt-2">
          <TraceWaterfall spans={data.rows ?? []} />
        </div>
      ) : (
        <div className="text-grafana-muted text-[11px] mt-2 italic">
          no data
        </div>
      )}
    </div>
  );
}
