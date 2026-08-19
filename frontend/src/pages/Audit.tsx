import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { AuditEntry, AuditStats } from "@/types/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

// Audit page: surfaces the per-write audit log.
export default function Audit() {
  const [entries, setEntries] = useState<AuditEntry[] | null>(null);
  const [stats, setStats] = useState<AuditStats | null>(null);
  const [err, setErr] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [a, s] = await Promise.all([api.audit(200), api.auditStats()]);
        if (cancelled) return;
        setEntries(a.entries);
        setStats(s);
      } catch (e) {
        if (!cancelled) setErr(e as Error);
      }
    };
    load();
    const t = setInterval(load, 10_000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, []);

  if (err) return <ErrorBox error={err} />;
  if (!entries || !stats) return <Skeleton rows={8} />;

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Audit log</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Every write request the API key performed.
          </p>
        </header>

        <section className="grid grid-cols-3 gap-4">
          <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3">
            <div className="text-xs uppercase text-grafana-muted">Total</div>
            <div className="text-2xl font-semibold mt-1">{stats.total}</div>
          </div>
          <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3">
            <div className="text-xs uppercase text-grafana-muted">OK</div>
            <div className="text-2xl font-semibold mt-1 text-emerald-400">{stats.ok}</div>
          </div>
          <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3">
            <div className="text-xs uppercase text-grafana-muted">Failed</div>
            <div className="text-2xl font-semibold mt-1 text-red-400">{stats.failed}</div>
          </div>
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">By action</h2>
          <div className="overflow-x-auto rounded border border-grafana-border">
            <table className="w-full text-sm">
              <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                <tr>
                  <th className="px-3 py-2 text-left">Action</th>
                  <th className="px-3 py-2 text-right">Count</th>
                </tr>
              </thead>
              <tbody>
                {Object.entries(stats.by_action).map(([k, v]) => (
                  <tr key={k} className="border-t border-grafana-border">
                    <td className="px-3 py-2 font-mono">{k}</td>
                    <td className="px-3 py-2 text-right">{v}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Recent entries ({entries.length})</h2>
          <div className="overflow-x-auto rounded border border-grafana-border">
            <table className="w-full text-sm">
              <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                <tr>
                  <th className="px-3 py-2 text-left">Timestamp</th>
                  <th className="px-3 py-2 text-left">Actor</th>
                  <th className="px-3 py-2 text-left">Action</th>
                  <th className="px-3 py-2 text-left">Target</th>
                  <th className="px-3 py-2 text-left">IP</th>
                  <th className="px-3 py-2 text-left">Result</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e, i) => (
                  <tr key={i} className="border-t border-grafana-border">
                    <td className="px-3 py-2 font-mono">{new Date(e.ts).toLocaleString()}</td>
                    <td className="px-3 py-2">{e.actor}</td>
                    <td className="px-3 py-2">{e.action}</td>
                    <td className="px-3 py-2 font-mono">{e.target || ""}</td>
                    <td className="px-3 py-2">{e.ip || ""}</td>
                    <td className={e.ok ? "px-3 py-2 text-emerald-400" : "px-3 py-2 text-red-400"}>
                      {e.ok ? "ok" : "failed: " + (e.error || "")}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </FadeIn>
  );
}
