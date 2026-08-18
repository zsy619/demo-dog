import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { MetricSeries, ServiceSummary, HistogramResponse } from "@/types/api";
import TimeSeriesChart from "@/components/charts/TimeSeriesChart";
import StatCard from "@/components/StatCard";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import { duration, sinceMs } from "@/lib/time";
import TimeRangePicker from "@/components/TimeRangePicker";
import { toast } from "@/components/Toast";
import { useHashState, useHashStateBool } from "@/hooks/useHashState";
import BarLoader from "@/components/anim/BarLoader";
import FadeIn from "@/components/anim/FadeIn";

interface Props {
  service: string;
  onServiceChange?: (s: string) => void;
}

const DEFAULT_METRICS = [
  "http.server.duration",
  "http.server.requests",
  "process.cpu",
  "system.mem.used",
];

export default function Metrics({ service, onServiceChange }: Props) {
  const [series, setSeries] = useState<MetricSeries[]>([]);
  // Filter state lives in the URL so refresh / deep-link share just work.
  const [metric, setMetric] = useHashState("metric", DEFAULT_METRICS[0]);
  const [windowSelRaw, setWindowSelRaw] = useHashState("window", "1m");
  const windowSel: "1m" | "5m" = windowSelRaw === "5m" ? "5m" : "1m";
  const setWindowSel = (v: "1m" | "5m") => setWindowSelRaw(v);
  const [range, setRange] = useHashState("range", "15m");
  const [histBins, setHistBins] = useHashState<number>("bins", 20,
    (raw) => Math.max(1, Math.min(100, parseInt(raw, 10) || 20)),
    (val) => String(val));
  const [histScaleRaw, setHistScaleRaw] = useHashState("scale", "linear");
  const histScale: "linear" | "log" = histScaleRaw === "log" ? "log" : "linear";
  const setHistScale = (v: "linear" | "log") => setHistScaleRaw(v);
  const [metricNames, setMetricNames] = useState<string[]>(DEFAULT_METRICS);
  const [services, setServices] = useState<ServiceSummary[]>([]);
  const [hist, setHist] = useState<HistogramResponse | null>(null);
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);
  // Compare mode: when on, the chart overlays every service on the same axis
  // so users can spot outliers at a glance.
  const [compare, setCompare] = useHashStateBool("compare", false);
  const [picked, setPicked] = useHashState<Set<string>>(
    "picked",
    new Set(),
    (raw) => new Set(raw.split(",").filter(Boolean)),
    (val) => Array.from(val).join(",")
  );
  // Free-form labels filter: env=prod region=us-east — same parsing style as
  // Logs. The merge happens inside reload() so the actual query is honest.
  const [labelsText, setLabelsText] = useHashState("labels", "");

  const reload = async () => {
    try {
      const labels: Record<string, string> = {};
      for (const piece of labelsText.split(/[\s,]+/).filter(Boolean)) {
        const [k, v] = piece.split("=");
        if (k && v) labels[k] = v;
      }
      const r = await api.query({
        type: "metrics",
        service,
        name: metric,
        window: windowSel,
        since: sinceMs(range as any),
        labels,
        limit: 600,
      });
      setSeries(r.series ?? []);
      setErr(null);
    } catch (e) {
      setErr(e as Error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    reload();
    const id = window.setInterval(reload, 5000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [service, metric, windowSel, range, labelsText]);

  useEffect(() => {
    api.metricNames(50).then((r) => setMetricNames(r.names)).catch(() => {});
    api.services().then((r) => setServices(r.services)).catch(() => {});
  }, []);

  useEffect(() => {
    if (!service) {
      setHist(null);
      return;
    }
    api.histogram(service, histBins).then(setHist).catch(() => {});
  }, [service, histBins]);

  // Drop null / NaN points before doing aggregates so a single bad
  // sample doesn’t surface as the top of the chart or crash the page.
  const flat = series
    .flatMap((s) => (s && s.points ? s.points : []))
    .filter(
      (p) =>
        p !== null &&
        p !== undefined &&
        typeof p.value === "number" &&
        Number.isFinite(p.value)
    );
  const last = flat.length > 0 ? flat[flat.length - 1].value : 0;
  const max = flat.length > 0 ? Math.max(...flat.map((p) => p.value)) : 0;
  const avg =
    flat.length > 0 ? flat.reduce((a, b) => a + b.value, 0) / flat.length : 0;

  const seed = async () => {
    try {
      const svc = service || "demo";
      const r = await api.seed(svc, 8);
      toast(`seeded ${r.seeded} events for ${r.service}`, "success");
      reload();
    } catch (e) {
      toast(`seed failed: ${(e as Error).message}`, "error");
    }
  };

  const fmt = (v: number) => (metric.includes("duration") ? duration(v) : v.toFixed(2));

  return (
    <FadeIn className="p-4 space-y-4">
      {err && <ErrorBox error={err} onRetry={reload} />}

      <div className="bg-grafana-panel border border-grafana-border rounded-lg p-3 flex flex-wrap items-center gap-2 text-xs">
        <input
          value={service}
          onChange={(e) => onServiceChange?.(e.target.value)}
          placeholder="service (all)"
          className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 w-40 focus:outline-none focus:border-grafana-blue"
        />
        <select
          value={metric}
          onChange={(e) => setMetric(e.target.value)}
          className="bg-grafana-elev border border-grafana-border rounded px-2 py-1"
        >
          {[...new Set([...DEFAULT_METRICS, ...metricNames])].map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
        <select
          value={windowSel}
          onChange={(e) => setWindowSel(e.target.value as "1m" | "5m")}
          className="bg-grafana-elev border border-grafana-border rounded px-2 py-1"
        >
          <option value="1m">window 1m</option>
          <option value="5m">window 5m</option>
        </select>
        <TimeRangePicker value={range} onChange={setRange} />
        <input
          value={labelsText}
          onChange={(e) => setLabelsText(e.target.value)}
          placeholder="labels: env=prod region=us-east"
          className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 w-56 focus:outline-none focus:border-grafana-blue"
          title="Filter metrics by label key=value pairs"
        />
        <label
          className={`px-2 py-1 rounded border cursor-pointer flex items-center gap-1.5 ${
            compare
              ? "bg-grafana-accent/15 border-grafana-accent text-grafana-accent"
              : "bg-grafana-elev border-grafana-border text-grafana-muted"
          }`}
          title="Overlay every service on the same chart"
        >
          <input
            type="checkbox"
            checked={compare}
            onChange={(e) => {
              setCompare(e.target.checked);
              if (!e.target.checked) setPicked(new Set());
            }}
          />
          compare all
        </label>
        <button
          onClick={reload}
          className="bg-grafana-elev border border-grafana-border rounded px-3 py-1 hover:bg-grafana-elev2"
        >
          ⟳ Refresh
        </button>
        <button
          onClick={seed}
          className="bg-grafana-blue/20 border border-grafana-blue/40 rounded px-3 py-1 text-grafana-blue"
        >
          + seed
        </button>
        <span className="ml-auto text-grafana-muted">
          {flat.length} points · {sinceMs(range as any) ? `last ${range}` : "all"}
        </span>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatCard title="Current" value={fmt(last)} accent="blue" />
        <StatCard title="Avg" value={fmt(avg)} accent="green" />
        <StatCard title="Max" value={fmt(max)} accent="purple" />
        <StatCard title="Points" value={flat.length} accent="amber" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
        <div className="lg:col-span-2 bg-grafana-panel border border-grafana-border rounded-lg p-4">
          <div className="flex items-center justify-between mb-3">
            <div>
              <div className="text-[13px] font-semibold">{metric}</div>
              <div className="text-[11px] text-grafana-muted">
                {service ? `service=${service}` : "all services"} · window={windowSel}
              </div>
            </div>
            <div className="text-[11px] text-grafana-muted">{flat.length} points</div>
          </div>
          {loading && flat.length === 0 ? (
            <FadeIn className="flex items-center justify-center py-8 gap-2">
              <BarLoader height={28} color="#3b82f6" />
              <span className="text-[11px] text-grafana-muted ml-2">
                resolving {histBins}-bin histogram…
              </span>
            </FadeIn>
          ) : flat.length === 0 ? (
            <div className="text-grafana-muted italic text-sm py-8 text-center">
              No metric points yet — try + seed.
            </div>
          ) : compare ? (
            <>
              <ComparePicker
                services={services}
                picked={picked}
                onChange={setPicked}
              />
              <CompareChart
                metric={metric}
                picked={picked}
                allServices={services}
                since={sinceMs(range as any)}
                windowSel={windowSel}
              />
            </>
          ) : (
            <TimeSeriesChart
              series={[{ name: metric, points: flat, color: "#3b82f6" }]}
              height={260}
              showLegend
            />
          )}
        </div>
        <div className="bg-grafana-panel border border-grafana-border rounded-lg p-4">
          <div className="flex items-center justify-between mb-2 flex-wrap gap-1">
            <div className="text-[13px] font-semibold">
              Latency histogram
              {service && (
                <span className="text-grafana-muted text-[11px] ml-1">({service})</span>
              )}
            </div>
            <div className="flex items-center gap-1 text-[10px]">
              <div className="inline-flex bg-grafana-elev border border-grafana-border rounded">
                {(["linear", "log"] as const).map((s) => (
                  <button
                    key={s}
                    onClick={() => setHistScale(s)}
                    className={`px-1.5 py-0.5 ${histScale === s ? "bg-grafana-accent/20 text-grafana-accent" : "text-grafana-muted"}`}
                  >
                    {s}
                  </button>
                ))}
              </div>
              <select
                value={histBins}
                onChange={(e) => setHistBins(Number(e.target.value))}
                className="bg-grafana-elev border border-grafana-border rounded px-1 py-0.5 text-grafana-text"
                title="Bucket count"
              >
                {[10, 20, 30, 50].map((b) => (
                  <option key={b} value={b}>
                    {b}b
                  </option>
                ))}
              </select>
            </div>
          </div>
          {!service ? (
            <div className="text-grafana-muted italic text-xs py-8 text-center">
              Select a service to see its latency distribution.
            </div>
          ) : hist ? (
            <>
              <Histogram
                bins={hist.counts}
                scale={histScale}
                maxValue={Math.max(...hist.counts, 1)}
              />
              <div className="grid grid-cols-3 gap-2 mt-3 text-xs">
                <StatCard title="p50" value={duration(hist.p50_ms)} accent="green" />
                <StatCard title="p95" value={duration(hist.p95_ms)} accent="warn" />
                <StatCard title="p99" value={duration(hist.p99_ms)} accent="err" />
              </div>
            </>
          ) : (
            <Skeleton rows={3} />
          )}
        </div>
      </div>

      <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[12px] text-grafana-muted">
          All services
        </div>
        <table className="w-full text-sm">
          <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
            <tr>
              <th className="px-3 py-2 text-left">Service</th>
              <th className="px-3 py-2 text-right">p50</th>
              <th className="px-3 py-2 text-right">p95</th>
              <th className="px-3 py-2 text-right">p99</th>
              <th className="px-3 py-2 text-right">QPS</th>
              <th className="px-3 py-2 text-right">Err</th>
              <th className="px-3 py-2 text-right">Labels</th>
            </tr>
          </thead>
          <tbody>
            {services.map((s) => (
              <tr
                key={s.name}
                onClick={() => onServiceChange?.(s.name)}
                className={`border-t border-grafana-border cursor-pointer hover:bg-grafana-elev/50 ${
                  service === s.name ? "bg-grafana-elev/30" : ""
                }`}
              >
                <td className="px-3 py-1.5 font-medium">{s.name}</td>
                <td className="px-3 py-1.5 text-right font-mono">{s.p50_ms.toFixed(0)}ms</td>
                <td className="px-3 py-1.5 text-right font-mono">{s.p95_ms.toFixed(0)}ms</td>
                <td className="px-3 py-1.5 text-right font-mono">{s.p99_ms.toFixed(0)}ms</td>
                <td className="px-3 py-1.5 text-right font-mono">{s.qps.toFixed(2)}</td>
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
                    {(s.error_rate * 100).toFixed(1)}%
                  </span>
                </td>
                <td className="px-3 py-1.5 text-right text-[11px] text-grafana-muted">
                  {(s.last_labels ?? []).slice(0, 3).join(", ") || "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </FadeIn>
  );
}

function Histogram({
  bins,
  maxValue,
  scale,
}: {
  bins: number[];
  maxValue: number;
  scale: "linear" | "log";
}) {
  if (!bins.length) {
    return (
      <div className="text-grafana-muted italic text-xs py-4">
        No samples yet.
      </div>
    );
  }
  const scaleFn = scale === "log" ? (v: number) => Math.log10(v + 1) : (v: number) => v;
  const m = Math.max(...bins.map(scaleFn), 1);
  return (
    <div className="flex items-end gap-0.5 h-28">
      {bins.map((c, i) => {
        const v = scaleFn(c);
        return (
          <div
            key={i}
            className="flex-1 bg-grafana-accent/60 hover:bg-grafana-accent rounded-t transition-all"
            style={{ height: `${Math.max(2, (v / m) * 100)}%` }}
            title={`bin ${i + 1}: ${c} samples`}
          />
        );
      })}
    </div>
  );
}


// ComparePicker renders a row of service chips that the user can toggle to
// pin/unpin a service in the compare-mode overlay chart.
function ComparePicker({
  services,
  picked,
  onChange,
}: {
  services: ServiceSummary[];
  picked: Set<string>;
  onChange: (next: Set<string>) => void;
}) {
  return (
    <div className="flex items-center gap-1.5 flex-wrap mb-2 text-[11px]">
      <span className="text-grafana-muted">services:</span>
      {services.map((s) => {
        const isOn = picked.has(s.name);
        return (
          <button
            key={s.name}
            onClick={() => {
              const next = new Set(picked);
              if (isOn) next.delete(s.name);
              else next.add(s.name);
              onChange(next);
            }}
            className={`px-1.5 py-0.5 rounded border transition-colors ${
              isOn
                ? "bg-grafana-accent/20 border-grafana-accent text-grafana-accent"
                : "bg-grafana-elev border-grafana-border text-grafana-muted hover:text-grafana-text"
            }`}
          >
            {s.name}
          </button>
        );
      })}
      <span className="ml-auto text-grafana-muted">
        {picked.size === 0 ? "(showing all)" : `${picked.size} selected`}
      </span>
    </div>
  );
}

// CompareChart fans out one query per picked service and overlays their
// metric series on the same TimeSeriesChart. Uses the store colours palette
// so each line is distinguishable.
const COMPARE_COLORS = [
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

function CompareChart({
  metric,
  picked,
  allServices,
  since,
  windowSel,
}: {
  metric: string;
  picked: Set<string>;
  allServices: ServiceSummary[];
  since: number;
  windowSel: "1m" | "5m";
}) {
  const targets =
    picked.size > 0
      ? allServices.filter((s) => picked.has(s.name))
      : allServices;
  const [combined, setCombined] = useState<
    Array<{ service: string; points: { ts: number; value: number }[] }>
  >([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all(
      targets.map((s) =>
        api
          .query({
            type: "metrics",
            service: s.name,
            name: metric,
            window: windowSel,
            since,
            limit: 600,
          })
          .then((r) => ({
            service: s.name,
            points: (r.series ?? []).flatMap((sr) => sr.points),
          }))
          .catch(() => ({ service: s.name, points: [] }))
      )
    ).then((rows) => {
      if (cancelled) return;
      setCombined(rows);
      setLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [targets.map((s) => s.name).join("|"), metric, windowSel, since]);

  const chartSeries = combined.map((r, i) => ({
    name: r.service,
    points: r.points,
    color: COMPARE_COLORS[i % COMPARE_COLORS.length],
  }));

  if (loading && chartSeries.length === 0) return <Skeleton rows={3} />;

  return (
    <TimeSeriesChart
      series={chartSeries}
      height={260}
      showLegend
      hideEmpty
    />
  );
}
