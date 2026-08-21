import { useMemo, useState } from "react";

import { api } from "@/lib/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";
import {
  currentPeriod,
  flattenUsageToRows,
  useBilling,
} from "@/hooks/useBilling";

export default function Billing() {
  const [tenant, setTenant] = useState("");
  const [period, setPeriod] = useState<string>(currentPeriod());
  const [csvBusy, setCsvBusy] = useState(false);
  const [csvErr, setCsvErr] = useState<Error | null>(null);

  const { rows, usage, loading, error, reload } = useBilling(tenant);

  // 表格行:按 period+tenant+metric 排序,前端重新聚合。
  const tableRows = useMemo(() => {
    if (tenant) return flattenUsageToRows(tenant, usage);
    return rows
      .map((r) => ({
        tenant: r.tenant,
        metric: r.metric,
        period: r.period,
        value: r.value,
        updatedAt: r.updated_at,
      }))
      .sort((a, b) => {
        if (a.period !== b.period) return b.period.localeCompare(a.period);
        if (a.tenant !== b.tenant) return a.tenant.localeCompare(b.tenant);
        return a.metric.localeCompare(b.metric);
      });
  }, [tenant, rows, usage]);

  const filtered = useMemo(() => {
    if (!period) return tableRows;
    return tableRows.filter((r) => r.period === period);
  }, [tableRows, period]);

  // 汇总(本页所显示行)
  const totals = useMemo(() => {
    const byMetric = new Map<string, number>();
    for (const r of filtered) {
      byMetric.set(r.metric, (byMetric.get(r.metric) ?? 0) + r.value);
    }
    return Array.from(byMetric.entries()).sort((a, b) => b[1] - a[1]);
  }, [filtered]);

  const downloadCsv = async () => {
    setCsvBusy(true);
    setCsvErr(null);
    try {
      const text = await api.billingUsageCSV({
        tenant: tenant || undefined,
        period: period || undefined,
      });
      // 浏览器侧触发下载。
      const blob = new Blob([text], { type: "text/csv;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      const stamp = new Date()
        .toISOString()
        .replace(/[:T]/g, "-")
        .replace(/\..*$/, "");
      a.href = url;
      a.download = `billing-${tenant || "all"}-${period || "all"}-${stamp}.csv`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (e) {
      setCsvErr(e as Error);
    } finally {
      setCsvBusy(false);
    }
  };

  if (error && !rows.length && !usage.length)
    return <ErrorBox error={error} />;
  if (loading && !rows.length && !usage.length) return <Skeleton rows={8} />;

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Billing</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Per-tenant usage across periods. Backend persists under{" "}
            <code>metering/&lt;period&gt;/&lt;tenant&gt;/&lt;metric&gt;</code>{" "}
            via the shared KV.
          </p>
        </header>

        <section className="flex flex-wrap items-end gap-3">
          <label className="flex flex-col text-xs text-grafana-muted">
            Tenant (blank = all)
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm min-w-[160px]"
              placeholder="acme"
              value={tenant}
              onChange={(e) => setTenant(e.target.value)}
            />
          </label>
          <label className="flex flex-col text-xs text-grafana-muted">
            Period (YYYY-MM, blank = all)
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm min-w-[120px]"
              placeholder="2026-03"
              value={period}
              onChange={(e) => setPeriod(e.target.value)}
            />
          </label>
          <button
            className="rounded bg-grafana-accent text-white px-3 py-2 text-sm hover:opacity-90 disabled:opacity-50"
            onClick={downloadCsv}
            disabled={csvBusy}
          >
            {csvBusy ? "Generating…" : "Download CSV"}
          </button>
          <button
            className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm hover:bg-grafana-panel-3"
            onClick={reload}
          >
            Reload
          </button>
        </section>

        {csvErr && (
          <ErrorBox
            error={csvErr}
            onRetry={() => setCsvErr(null)}
          />
        )}

        <section>
          <h2 className="text-lg font-medium mb-2">Totals (filtered)</h2>
          {totals.length === 0 ? (
            <p className="text-sm text-grafana-muted">
              No usage recorded for this filter.
            </p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {totals.map(([m, v]) => (
                <span
                  key={m}
                  className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-1 text-sm"
                >
                  <span className="text-grafana-muted mr-2">{m}</span>
                  <span className="font-mono">{v.toLocaleString()}</span>
                </span>
              ))}
            </div>
          )}
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Per-bucket rows</h2>
          {filtered.length === 0 ? (
            <p className="text-sm text-grafana-muted">No rows match filter.</p>
          ) : (
            <div className="overflow-x-auto rounded border border-grafana-border">
              <table className="min-w-full text-sm">
                <thead className="bg-grafana-panel-2 text-grafana-muted">
                  <tr>
                    <th className="text-left px-3 py-2">Tenant</th>
                    <th className="text-left px-3 py-2">Metric</th>
                    <th className="text-left px-3 py-2">Period</th>
                    <th className="text-right px-3 py-2">Value</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((r, idx) => (
                    <tr
                      key={`${r.tenant}-${r.metric}-${r.period}-${idx}`}
                      className="border-t border-grafana-border"
                    >
                      <td className="px-3 py-2 font-mono">{r.tenant}</td>
                      <td className="px-3 py-2 font-mono">{r.metric}</td>
                      <td className="px-3 py-2 font-mono">{r.period}</td>
                      <td className="px-3 py-2 text-right">
                        {r.value.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
    </FadeIn>
  );
}
