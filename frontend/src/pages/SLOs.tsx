import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { SLOBudget } from "@/types/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

export default function SLOs() {
  const [slos, setSlos] = useState<SLOBudget[] | null>(null);
  const [err, setErr] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const r = await api.slos();
        if (!cancelled) setSlos(r.slos);
      } catch (e) {
        if (!cancelled) setErr(e as Error);
      }
    };
    load();
    const t = setInterval(load, 30_000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, []);

  if (err) return <ErrorBox error={err} />;
  if (!slos) return <Skeleton rows={4} />;

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">SLO budgets</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Per-service availability / latency error budgets.
          </p>
        </header>

        {slos.length === 0 ? (
          <p className="text-sm text-grafana-muted">No SLOs registered.</p>
        ) : (
          <div className="overflow-x-auto rounded border border-grafana-border">
            <table className="w-full text-sm">
              <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                <tr>
                  <th className="px-3 py-2 text-left">Name</th>
                  <th className="px-3 py-2 text-left">Service</th>
                  <th className="px-3 py-2 text-right">Target</th>
                  <th className="px-3 py-2 text-right">Error rate</th>
                  <th className="px-3 py-2 text-right">Budget left</th>
                  <th className="px-3 py-2 text-left">Status</th>
                </tr>
              </thead>
              <tbody>
                {slos.map((s) => (
                  <tr key={s.name} className="border-t border-grafana-border">
                    <td className="px-3 py-2 font-mono">{s.name}</td>
                    <td className="px-3 py-2">{s.service}</td>
                    <td className="px-3 py-2 text-right">{(s.target * 100).toFixed(3)}%</td>
                    <td className="px-3 py-2 text-right">{(s.error_rate * 100).toFixed(3)}%</td>
                    <td className="px-3 py-2 text-right">{s.budget_left_percent.toFixed(2)}%</td>
                    <td className={s.healthy ? "px-3 py-2 text-emerald-400" : "px-3 py-2 text-red-400"}>
                      {s.healthy ? "healthy" : "burning"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </FadeIn>
  );
}
