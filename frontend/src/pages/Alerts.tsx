import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { AlertRule, AlertFire } from "@/types/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

// Alerts page: shows the active SLO rules and the last N fires.
// Pure read-only for now — adding / editing rules is CLI-only
// because the rules are intended to live in version control.
export default function Alerts() {
  const [rules, setRules] = useState<AlertRule[] | null>(null);
  const [fires, setFires] = useState<AlertFire[] | null>(null);
  const [err, setErr] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [r, f] = await Promise.all([api.alertsRules(), api.alertsFires(50)]);
        if (cancelled) return;
        setRules(r.rules);
        setFires(f.fires);
      } catch (e) {
        if (!cancelled) setErr(e as Error);
      }
    };
    load();
    const t = setInterval(load, 15_000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, []);

  if (err) {
    return <ErrorBox error={err} />;
  }
  if (!rules || !fires) {
    return <Skeleton rows={8} />;
  }

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Alerts</h1>
          <p className="text-sm text-grafana-muted mt-1">
            SLO burn-rate rules and recent fires. Rules are loaded from
            the collectors -alerts-rules flag; edit the YAML in git to
            change them.
          </p>
        </header>

        <section>
          <h2 className="text-lg font-medium mb-2">Active rules ({rules.length})</h2>
          {rules.length === 0 ? (
            <p className="text-sm text-grafana-muted">
              No rules configured. Start the collector with -alerts-rules.
            </p>
          ) : (
            <div className="overflow-x-auto rounded border border-grafana-border">
              <table className="w-full text-sm">
                <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                  <tr>
                    <th className="px-3 py-2 text-left">Name</th>
                    <th className="px-3 py-2 text-left">Service</th>
                    <th className="px-3 py-2 text-right">Target</th>
                    <th className="px-3 py-2 text-right">Fast burn</th>
                    <th className="px-3 py-2 text-right">Slow burn</th>
                    <th className="px-3 py-2 text-left">Severity</th>
                  </tr>
                </thead>
                <tbody>
                  {rules.map((r) => (
                    <tr key={r.name} className="border-t border-grafana-border">
                      <td className="px-3 py-2 font-mono">{r.name}</td>
                      <td className="px-3 py-2">{r.service || "*"}</td>
                      <td className="px-3 py-2 text-right">{(r.target * 100).toFixed(2)}%</td>
                      <td className="px-3 py-2 text-right">{r.fast_burn}x</td>
                      <td className="px-3 py-2 text-right">{r.slow_burn}x</td>
                      <td className="px-3 py-2">
                        <span
                          className={
                            r.severity === "critical"
                              ? "text-red-400"
                              : r.severity === "warning"
                              ? "text-amber-400"
                              : "text-sky-400"
                          }
                        >
                          {r.severity}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Recent fires ({fires.length})</h2>
          {fires.length === 0 ? (
            <p className="text-sm text-grafana-muted">
              No fires in the current ring buffer. The buffer holds up to
              256 of the most recent fires across all rules.
            </p>
          ) : (
            <ul className="space-y-2">
              {fires
                .slice()
                .reverse()
                .map((f, i) => (
                  <li
                    key={i}
                    className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm"
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <span
                        className={
                          f.severity === "critical"
                            ? "text-red-400 font-semibold"
                            : f.severity === "warning"
                            ? "text-amber-400 font-semibold"
                            : "text-sky-400"
                        }
                      >
                        {f.severity.toUpperCase()}
                      </span>
                      <span className="font-mono">{f.rule.name}</span>
                      <span className="text-grafana-muted text-xs">
                        {new Date(f.timestamp).toLocaleString()} ({f.window} window)
                      </span>
                    </div>
                    <p className="text-xs text-grafana-muted">
                      burn {f.burn_rate.toFixed(2)}x — {f.reason}
                    </p>
                  </li>
                ))}
            </ul>
          )}
        </section>
      </div>
    </FadeIn>
  );
}
