import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

// Admin API keys page: list, mint, rotate, revoke global API keys
// stored in the backend admin store. The collector accepts the same
// value via either Authorization: Bearer or X-API-Key.
export default function AdminKeys() {
  const [keys, setKeys] = useState<Array<{ id: string; label: string; tenant: string; role: string; created_at: string }> | null>(null);
  const [draft, setDraft] = useState({ label: "", tenant: "", role: "admin", scopes: "", ttl_days: "" });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<Error | null>(null);
  const [lastCreated, setLastCreated] = useState<string | null>(null);

  const load = async () => {
    try {
      const r = await api.adminKeys();
      setKeys(r.keys);
    } catch (e) {
      setErr(e as Error);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const create = async () => {
    if (!draft.label || !draft.tenant) return;
    setBusy(true);
    setErr(null);
    try {
      const ttlNs = draft.ttl_days ? Number(draft.ttl_days) * 86400 * 1e9 : 0;
      const r = await api.createAdminKey({
        label: draft.label,
        tenant: draft.tenant,
        role: draft.role,
        scopes: draft.scopes.split(",").map((s) => s.trim()).filter(Boolean),
        ttl_ns: ttlNs,
      });
      setLastCreated(r.plaintext);
      setDraft({ label: "", tenant: "", role: "admin", scopes: "", ttl_days: "" });
      await load();
    } catch (e) {
      setErr(e as Error);
    } finally {
      setBusy(false);
    }
  };

  const rotate = async (id: string) => {
    setBusy(true);
    try {
      const r = await api.rotateAdminKey(id, 0);
      setLastCreated(r.plaintext);
      await load();
    } catch (e) {
      setErr(e as Error);
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (id: string) => {
    setBusy(true);
    try {
      await api.revokeAdminKey(id);
      await load();
    } catch (e) {
      setErr(e as Error);
    } finally {
      setBusy(false);
    }
  };

  if (err && !keys) return <ErrorBox error={err} />;
  if (!keys) return <Skeleton rows={4} />;

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Admin keys</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Global API keys for the collector. Bearer tokens stamped
            into the Authorization header.
          </p>
        </header>

        {lastCreated && (
          <div className="rounded border border-amber-500 bg-amber-500/10 p-3 text-sm space-y-1">
            <div className="font-medium text-amber-400">Plaintext (copy now, never shown again)</div>
            <code className="block bg-grafana-panel-2 px-2 py-1 rounded break-all">{lastCreated}</code>
            <button onClick={() => setLastCreated(null)} className="text-xs text-grafana-muted hover:underline">Dismiss</button>
          </div>
        )}

        <section>
          <h2 className="text-lg font-medium mb-2">Mint key</h2>
          <div className="flex gap-2 flex-wrap">
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[120px]"
              placeholder="label"
              value={draft.label}
              onChange={(e) => setDraft({ ...draft, label: e.target.value })}
            />
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[120px]"
              placeholder="tenant"
              value={draft.tenant}
              onChange={(e) => setDraft({ ...draft, tenant: e.target.value })}
            />
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[120px]"
              placeholder="role (admin|writer|reader)"
              value={draft.role}
              onChange={(e) => setDraft({ ...draft, role: e.target.value })}
            />
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[160px]"
              placeholder="scopes, comma separated"
              value={draft.scopes}
              onChange={(e) => setDraft({ ...draft, scopes: e.target.value })}
            />
            <input
              type="number"
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm w-24"
              placeholder="ttl days"
              value={draft.ttl_days}
              onChange={(e) => setDraft({ ...draft, ttl_days: e.target.value })}
            />
            <button
              disabled={busy || !draft.label || !draft.tenant}
              onClick={create}
              className="rounded bg-grafana-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
            >
              Mint
            </button>
          </div>
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Existing keys ({keys.length})</h2>
          {keys.length === 0 ? (
            <p className="text-sm text-grafana-muted">No keys yet.</p>
          ) : (
            <div className="overflow-x-auto rounded border border-grafana-border">
              <table className="w-full text-sm">
                <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                  <tr>
                    <th className="px-3 py-2 text-left">ID</th>
                    <th className="px-3 py-2 text-left">Label</th>
                    <th className="px-3 py-2 text-left">Tenant</th>
                    <th className="px-3 py-2 text-left">Role</th>
                    <th className="px-3 py-2 text-left">Created</th>
                    <th className="px-3 py-2 text-right" />
                  </tr>
                </thead>
                <tbody>
                  {keys.map((k) => (
                    <tr key={k.id} className="border-t border-grafana-border">
                      <td className="px-3 py-2 font-mono">{k.id}</td>
                      <td className="px-3 py-2">{k.label}</td>
                      <td className="px-3 py-2">{k.tenant}</td>
                      <td className="px-3 py-2">{k.role}</td>
                      <td className="px-3 py-2">{new Date(k.created_at).toLocaleString()}</td>
                      <td className="px-3 py-2 text-right space-x-2">
                        <button
                          disabled={busy}
                          onClick={() => rotate(k.id)}
                          className="text-sky-400 hover:underline text-xs"
                        >
                          Rotate
                        </button>
                        <button
                          disabled={busy}
                          onClick={() => revoke(k.id)}
                          className="text-red-400 hover:underline text-xs"
                        >
                          Revoke
                        </button>
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
