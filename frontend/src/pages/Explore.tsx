import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { QueryResult, ServiceSummary } from "@/types/api";
import TimeSeriesChart from "@/components/charts/TimeSeriesChart";
import Table from "@/components/Table";
import SeverityBadge from "@/components/SeverityBadge";
import TraceWaterfall from "@/components/charts/TraceWaterfall";
import SearchBox from "@/components/SearchBox";
import { ErrorBox } from "@/components/Feedback";
import Highlight from "@/components/Highlight";

interface Props {
  service: string;
  signal: "logs" | "metrics" | "traces";
  onSignalChange: (s: "logs" | "metrics" | "traces") => void;
  onServiceChange: (s: string) => void;
  onFilterChange?: (key: string, value: string) => void;
}

const METRIC_NAMES = [
  { value: "http.server.duration", label: "http.server.duration (ms)" },
  { value: "http.server.requests", label: "http.server.requests (count)" },
  { value: "process.cpu", label: "process.cpu (%)" },
  { value: "system.mem.used", label: "system.mem.used (bytes)" },
];

const SIGNAL_TABS: Array<{ id: Props["signal"]; label: string }> = [
  { id: "logs", label: "Logs" },
  { id: "metrics", label: "Metrics" },
  { id: "traces", label: "Traces" },
];

export default function Explore({
  service: initialService,
  signal,
  onSignalChange,
  onServiceChange,
  onFilterChange,
}: Props) {
  const [services, setServices] = useState<ServiceSummary[]>([]);
  const [service, setService] = useState(initialService);
  const [metricName, setMetricName] = useState(METRIC_NAMES[0].value);
  const [windowSel, setWindowSel] = useState<"1m" | "5m">("1m");
  const [limit, setLimit] = useState(200);
  const [search, setSearch] = useState("");
  const [traceId, setTraceId] = useState("");
  const [severity, setSeverity] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);

  useEffect(() => setService(initialService), [initialService]);

  // Initialize filter state from URL on mount.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const h = window.location.hash;
    const qIdx = h.indexOf("?");
    if (qIdx < 0) return;
    const params = new URLSearchParams(h.slice(qIdx + 1));
    const s = params.get("search");
    if (s !== null) setSearch(s);
    const t = params.get("trace_id");
    if (t !== null) setTraceId(t);
    const sev = params.get("severity");
    if (sev !== null) setSeverity(sev);
    const m = params.get("metric");
    if (m !== null) setMetricName(m);
    const w = params.get("window");
    if (w === "1m" || w === "5m") setWindowSel(w);
  }, []);

  // Push filter changes back into the URL so deep links survive reloads.
  useEffect(() => {
    if (!onFilterChange) return;
    if (search) onFilterChange("search", search);
    if (traceId) onFilterChange("trace_id", traceId);
    if (severity) onFilterChange("severity", severity);
    if (metricName) onFilterChange("metric", metricName);
    if (windowSel) onFilterChange("window", windowSel);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, traceId, severity, metricName, windowSel]);

  useEffect(() => {
    api.services().then((r) => setServices(r.services)).catch(() => {});
    api.metricNames(20).then((r) => {
      if (r.names.length && !METRIC_NAMES.some((m) => m.value === metricName)) {
        setMetricName(r.names[0]);
      }
    }).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const runQuery = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.query({
        type: signal,
        service,
        name: signal === "metrics" ? metricName : undefined,
        window: signal === "metrics" ? windowSel : undefined,
        trace_id: signal === "traces" ? traceId : undefined,
        severity: signal === "logs" ? severity : undefined,
        search: signal !== "metrics" ? search : undefined,
        limit,
      });
      setResult(res);
    } catch (e) {
      setError(String((e as Error).message ?? e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    runQuery();
    if (!autoRefresh) return;
    const id = window.setInterval(runQuery, 6000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signal, service, metricName, windowSel, traceId, severity, search, limit]);

  const serviceOptions = useMemo(() => services.map((s) => s.name), [services]);

  return (
    <div className="h-full grid grid-cols-12 gap-3 p-3">
      <section className="col-span-12 bg-grafana-panel border border-grafana-border rounded-lg p-4 flex flex-col gap-3">
        <div className="flex items-center gap-2 text-xs flex-wrap">
          <span className="text-grafana-muted">DOG / Demo /</span>
          <select
            value={service}
            onChange={(e) => {
              setService(e.target.value);
              onServiceChange(e.target.value);
            }}
            className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 text-grafana-text"
          >
            <option value="">all services</option>
            {serviceOptions.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
          <div className="inline-flex bg-grafana-elev border border-grafana-border rounded">
            {SIGNAL_TABS.map((t) => (
              <button
                key={t.id}
                onClick={() => onSignalChange(t.id)}
                className={
                  "px-3 py-1 text-xs " +
                  (signal === t.id
                    ? "bg-grafana-accent/20 text-grafana-accent"
                    : "text-grafana-muted hover:text-grafana-text")
                }
              >
                {t.label}
              </button>
            ))}
          </div>
          <label className="text-grafana-muted flex items-center gap-1 ml-auto">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            auto-refresh
          </label>
        </div>

        <div className="bg-grafana-elev border border-grafana-border rounded">
          <div className="flex items-center justify-between px-3 py-1.5 border-b border-grafana-border text-[11px] text-grafana-muted flex-wrap gap-2">
            <span className="flex items-center gap-3 flex-wrap font-mono">
              <span>SELECT * FROM {signal}</span>
              {signal === "metrics" && (
                <>
                  <span>
                    WHERE name ={" "}
                    <select
                      value={metricName}
                      onChange={(e) => setMetricName(e.target.value)}
                      className="bg-grafana-panel border border-grafana-border rounded px-1 py-0.5 text-grafana-text text-[11px]"
                    >
                      {[...METRIC_NAMES.map((m) => m.value), ...(services.length ? [] : [])]
                        .filter((v, i, a) => a.indexOf(v) === i)
                        .map((v) => (
                          <option key={v} value={v}>
                            {v}
                          </option>
                        ))}
                    </select>
                  </span>
                  <span>
                    window{" "}
                    <select
                      value={windowSel}
                      onChange={(e) =>
                        setWindowSel(e.target.value as "1m" | "5m")
                      }
                      className="bg-grafana-panel border border-grafana-border rounded px-1 py-0.5 text-grafana-text text-[11px]"
                    >
                      <option value="1m">1m</option>
                      <option value="5m">5m</option>
                    </select>
                  </span>
                </>
              )}
              {signal === "traces" && (
                <span>
                  WHERE trace_id ={" "}
                  <input
                    value={traceId}
                    onChange={(e) => setTraceId(e.target.value)}
                    placeholder="(any)"
                    className="bg-grafana-panel border border-grafana-border rounded px-1 py-0.5 text-grafana-text text-[11px] w-32"
                  />
                </span>
              )}
              {signal === "logs" && (
                <>
                  <span>
                    severity{" "}
                    <select
                      value={severity}
                      onChange={(e) => setSeverity(e.target.value)}
                      className="bg-grafana-panel border border-grafana-border rounded px-1 py-0.5 text-grafana-text text-[11px]"
                    >
                      <option value="">all</option>
                      {["TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"].map((s) => (
                        <option key={s} value={s}>
                          {s}
                        </option>
                      ))}
                    </select>
                  </span>
                  <div className="w-56">
                    <SearchBox
                      value={search}
                      onChange={setSearch}
                      placeholder="search body"
                    />
                  </div>
                </>
              )}
            </span>
            <span className="flex items-center gap-2">
              <label>limit</label>
              <input
                type="number"
                value={limit}
                onChange={(e) => setLimit(Math.max(1, Number(e.target.value)))}
                className="w-16 bg-grafana-panel border border-grafana-border rounded px-1 py-0.5 text-grafana-text text-[11px]"
              />
              <button
                onClick={runQuery}
                disabled={loading}
                className="bg-grafana-accent text-white px-3 py-1 rounded text-[11px] hover:bg-grafana-accent/80 disabled:opacity-50"
              >
                {loading ? "Running…" : "Run query"}
              </button>
            </span>
          </div>
        </div>

        {error && <ErrorBox error={new Error(error)} onRetry={runQuery} />}

        {result && (
          <div className="text-[11px] text-grafana-muted flex items-center gap-3 flex-wrap">
            <span>
              {result.stats.returned} rows · scanned {result.stats.scanned} ·{" "}
              {result.stats.took_ms}ms
            </span>
            <span className="text-grafana-border">|</span>
            <span>
              tier <span className="text-grafana-text">{result.stats.tier}</span>
            </span>
            {result.stats.mv_used && (
              <>
                <span className="text-grafana-border">|</span>
                <span>
                  mv_used{" "}
                  <span className="text-grafana-text">{result.stats.mv_used}</span>
                </span>
              </>
            )}
          </div>
        )}
      </section>

      <section className="col-span-12 bg-grafana-panel border border-grafana-border rounded-lg p-4">
        {!result ? (
          <div className="text-grafana-muted text-sm italic">
            Run a query to see results.
          </div>
        ) : signal === "metrics" ? (
          <div className="space-y-3">
            {(result.series ?? []).map((s) => (
              <div
                key={s.service + s.name}
                className="bg-grafana-elev border border-grafana-border rounded p-3"
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="text-[13px] font-semibold">
                    {s.name}{" "}
                    <span className="text-grafana-muted font-normal">
                      ({s.service || "all"})
                    </span>
                  </div>
                  <div className="text-[11px] text-grafana-muted">
                    {(s.points ?? []).length} points
                  </div>
                </div>
                <TimeSeriesChart
                  series={[{ name: s.name, points: s.points, color: "#3b82f6" }]}
                  height={200}
                />
              </div>
            ))}
            {(result.series ?? []).length === 0 && (
              <div className="text-grafana-muted text-sm italic">
                no data in this window
              </div>
            )}
          </div>
        ) : signal === "logs" ? (
          <Table
            rows={result.rows ?? []}
            columns={[
              { key: "timestamp", label: "Time", width: "200px", mono: true },
              { key: "service", label: "Service", width: "120px" },
              {
                key: "severity",
                label: "Level",
                width: "90px",
                render: (r) => (
                  <SeverityBadge value={String(r.severity ?? "INFO")} />
                ),
              },
              {
                key: "body",
                label: "Message",
                render: (r) => (
                  <Highlight text={String(r.body ?? "")} query={search} />
                ),
              },
              { key: "trace_id", label: "Trace", width: "120px", mono: true },
            ]}
            emptyHint="no log records found"
            maxHeight={500}
          />
        ) : (
          <TraceWaterfall spans={result.rows ?? []} />
        )}
      </section>
    </div>
  );
}
