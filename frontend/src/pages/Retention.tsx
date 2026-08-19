import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { RetentionPolicy } from "@/types/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

const NS_PER_HOUR = 3600 * 1e9;
const NS_PER_DAY = 24 * NS_PER_HOUR;

export default function Retention() {
  const [policies, setPolicies] = useState<RetentionPolicy[] | null>(null);
  const [report, setReport] = useState<any | null>(null);
  const [tenant, setTenant] = useState("");
  const [err, setErr] = useState<Error | null>(null);
  const [draft, setDraft] = useState<RetentionPolicy>({
    tenant: "",
    tier: "pro",
    hot_ttl_ns: 7 * NS_PER_DAY,
    cold_ttl_ns: 30 * NS_PER_DAY,
    updated_at: "",
  });

  const load = async () => {
    try {
      const r = await api.retentionList();
      setPolicies(r.policies);
    } catch (e) {
      setErr(e as Error);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const save = async () => {
    if (!draft.tenant) return;
    try {
      await api.setRetention(draft);
      setDraft({ ...draft, tenant: "" });
      await load();
    } catch (e) {
      setErr(e as Error);
    }
  };

  const runReport = async () => {
    if (!tenant) return;
    try {
      const r = await api.retentionReport(tenant);
      setReport(r);
    } catch (e) {
      setErr(e as Error);
    }
  };

  if (err && !policies) return <ErrorBox error={err} />;
  if (!policies) return <Skeleton rows={4} />;

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Retention</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Per-tenant hot / cold storage policies.
          </p>
        </header>

        <section>
          <h2 className="text-lg font-medium mb-2">Existing policies ({policies.length})</h2>
          {policies.length === 0 ? (
            <p className="text-sm text-grafana-muted">No custom policies set.</p>
          ) : (
            <div className="overflow-x-auto rounded border border-grafana-border">
              <table className="w-full text-sm">
                <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                  <tr>
                    <th className="px-3 py-2 text-left">Tenant</th>
                    <th className="px-3 py-2 text-left">Tier</th>
                    <th className="px-3 py-2 text-right">Hot TTL</th>
                    <th className="px-3 py-2 text-right">Cold TTL</th>
                    <th className="px-3 py-2 text-left">Updated</th>
                  </tr>
                </thead>
                <tbody>
                  {policies.map((p) => (
                    <tr key={p.tenant} className="border-t border-grafana-border">
                      <td className="px-3 py-2 font-mono">{p.tenant}</td>
                      <td className="px-3 py-2">{p.tier}</td>
                      <td className="px-3 py-2 text-right">{Math.round(p.hot_ttl_ns / NS_PER_DAY)}d</td>
                      <td className="px-3 py-2 text-right">{Math.round(p.cold_ttl_ns / NS_PER_DAY)}d</td>
                      <td className="px-3 py-2">{new Date(p.updated_at).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Upsert policy</h2>
          <div className="flex gap-2 flex-wrap">
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[160px]"
              placeholder="tenant id"
              value={draft.tenant}
              onChange={(e) => setDraft({ ...draft, tenant: e.target.value })}
            />
            <select
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm"
              value={draft.tier}
              onChange={(e) => setDraft({ ...draft, tier: e.target.value as any })}
            >
              <option value="free">free</option>
              <option value="pro">pro</option>
              <option value="enterprise">enterprise</option>
            </select>
            <input
              type="number"
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm w-32"
              placeholder="hot days"
              value={Math.round(draft.hot_ttl_ns / NS_PER_DAY)}
              onChange={(e) => setDraft({ ...draft, hot_ttl_ns: Number(e.target.value) * NS_PER_DAY })}
            />
            <input
              type="number"
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm w-32"
              placeholder="cold days"
              value={Math.round(draft.cold_ttl_ns / NS_PER_DAY)}
              onChange={(e) => setDraft({ ...draft, cold_ttl_ns: Number(e.target.value) * NS_PER_DAY })}
            />
            <button
              disabled={!draft.tenant}
              onClick={save}
              className="rounded bg-grafana-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
            >
              Save
            </button>
          </div>
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Tenant report</h2>
          <div className="flex gap-2">
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1"
              placeholder="tenant id"
              value={tenant}
              onChange={(e) => setTenant(e.target.value)}
            />
            <button
              disabled={!tenant}
              onClick={runReport}
              className="rounded bg-grafana-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
            >
              Run
            </button>
          </div>
          {report && (
            <div className="mt-3 rounded border border-grafana-border bg-grafana-panel-2 p-3 text-sm">
              <pre className="whitespace-pre-wrap">{JSON.stringify(report, null, 2)}</pre>
            </div>
          )}
        </section>
      </div>
    </FadeIn>
  );
}
