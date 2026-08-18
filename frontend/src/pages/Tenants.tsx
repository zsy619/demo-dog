import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Tenant } from "@/types/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

// Admin-only page: create tenants, mint API keys, see which keys exist.
// Round 23.4 ships this as a read/write UI; the server enforces
// admin-only via the role gate.
export default function Tenants() {
  const [tenants, setTenants] = useState<Tenant[] | null>(null);
  const [err, setErr] = useState<Error | null>(null);
  const [newId, setNewId] = useState("");
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);
  const [mintedKey, setMintedKey] = useState<{ tenant_id: string; plaintext: string; role: string } | null>(null);
  const [mintTenant, setMintTenant] = useState("");
  const [mintLabel, setMintLabel] = useState("");
  const [mintRole, setMintRole] = useState("writer");

  const load = async () => {
    try {
      const r = await api.tenantsList();
      setTenants(r.tenants);
    } catch (e) {
      setErr(e as Error);
    }
  };

  useEffect(() => {
    load();
  }, []);

  if (err) return <ErrorBox error={err} />;
  if (!tenants) return <Skeleton rows={5} />;

  const create = async () => {
    if (!newId.trim()) return;
    setCreating(true);
    try {
      await api.createTenant({ id: newId, name: newName });
      setNewId("");
      setNewName("");
      await load();
    } catch (e) {
      setErr(e as Error);
    } finally {
      setCreating(false);
    }
  };

  const mint = async () => {
    if (!mintTenant.trim()) return;
    try {
      const k = await api.mintTenantKey(mintTenant, { label: mintLabel || "default", role: mintRole });
      setMintedKey({ tenant_id: k.tenant_id, plaintext: k.plaintext, role: k.role });
      setMintLabel("");
    } catch (e) {
      setErr(e as Error);
    }
  };

  return (
    <FadeIn>
      <div className="p-6 space-y-6 max-w-4xl">
        <header>
          <h1 className="text-2xl font-semibold">Tenants</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Each tenant gets its own slice of logs, metrics, and traces.
            API keys minted here are bound to a tenant and cannot read
            data from other tenants.
          </p>
        </header>

        <section className="rounded border border-grafana-border bg-grafana-panel-2 p-4">
          <h2 className="text-lg font-medium mb-3">Create tenant</h2>
          <div className="flex gap-2">
            <input
              aria-label="Tenant ID"
              placeholder="acme"
              value={newId}
              onChange={(e) => setNewId(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))}
              className="bg-grafana-bg border border-grafana-border rounded px-2 py-1 text-sm"
            />
            <input
              aria-label="Tenant name"
              placeholder="Acme Corp"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              className="bg-grafana-bg border border-grafana-border rounded px-2 py-1 text-sm flex-1"
            />
            <button
              onClick={create}
              disabled={creating || !newId.trim()}
              className="px-3 py-1 rounded bg-grafana-accent text-white text-sm disabled:opacity-50"
            >
              {creating ? "Creating..." : "Create"}
            </button>
          </div>
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Active tenants ({tenants.length})</h2>
          {tenants.length === 0 ? (
            <p className="text-sm text-grafana-muted">No tenants yet. Use the form above to add one.</p>
          ) : (
            <div className="overflow-x-auto rounded border border-grafana-border">
              <table className="w-full text-sm">
                <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                  <tr>
                    <th className="px-3 py-2 text-left">ID</th>
                    <th className="px-3 py-2 text-left">Name</th>
                    <th className="px-3 py-2 text-left">Created</th>
                    <th className="px-3 py-2 text-left">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {tenants.map((t) => (
                    <tr key={t.id} className="border-t border-grafana-border">
                      <td className="px-3 py-2 font-mono">{t.id}</td>
                      <td className="px-3 py-2">{t.name}</td>
                      <td className="px-3 py-2 text-grafana-muted">{new Date(t.created_at).toLocaleString()}</td>
                      <td className="px-3 py-2">
                        <span className={t.active ? "text-grafana-ok" : "text-grafana-muted"}>
                          {t.active ? "active" : "disabled"}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="rounded border border-grafana-border bg-grafana-panel-2 p-4">
          <h2 className="text-lg font-medium mb-3">Mint API key</h2>
          <div className="flex gap-2 mb-3">
            <select
              value={mintTenant}
              onChange={(e) => setMintTenant(e.target.value)}
              className="bg-grafana-bg border border-grafana-border rounded px-2 py-1 text-sm"
            >
              <option value="">Select tenant</option>
              {tenants.map((t) => (
                <option key={t.id} value={t.id}>{t.id}</option>
              ))}
            </select>
            <input
              aria-label="Key label"
              placeholder="checkout"
              value={mintLabel}
              onChange={(e) => setMintLabel(e.target.value)}
              className="bg-grafana-bg border border-grafana-border rounded px-2 py-1 text-sm flex-1"
            />
            <select
              value={mintRole}
              onChange={(e) => setMintRole(e.target.value)}
              className="bg-grafana-bg border border-grafana-border rounded px-2 py-1 text-sm"
            >
              <option value="reader">reader</option>
              <option value="writer">writer</option>
              <option value="admin">admin</option>
            </select>
            <button
              onClick={mint}
              disabled={!mintTenant}
              className="px-3 py-1 rounded bg-grafana-accent text-white text-sm disabled:opacity-50"
            >
              Mint
            </button>
          </div>
          {mintedKey && (
            <div className="bg-grafana-bg border border-grafana-border rounded p-3 text-xs font-mono break-all">
              <div className="text-grafana-muted mb-1">
                {mintedKey.tenant_id} / {mintedKey.role} — copy now, this is the only time the plaintext is shown:
              </div>
              {mintedKey.plaintext}
            </div>
          )}
        </section>
      </div>
    </FadeIn>
  );
}
