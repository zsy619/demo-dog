import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { WebhookSubscriber, WebhookStats, WebhookDelivery } from "@/types/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

export default function Webhooks() {
  const [subs, setSubs] = useState<WebhookSubscriber[] | null>(null);
  const [dlq, setDlq] = useState<WebhookDelivery[] | null>(null);
  const [stats, setStats] = useState<WebhookStats | null>(null);
  const [err, setErr] = useState<Error | null>(null);
  const [draft, setDraft] = useState({ url: "", secret: "", event_types: "" });
  const [busy, setBusy] = useState(false);

  const load = async () => {
    try {
      const [s, d, st] = await Promise.all([
        api.webhookSubscribers(),
        api.webhookDLQ(),
        api.webhookStats(),
      ]);
      setSubs(s.subscribers);
      setDlq(d.deliveries);
      setStats(st);
    } catch (e) {
      setErr(e as Error);
    }
  };

  useEffect(() => {
    load();
    const t = setInterval(load, 10_000);
    return () => clearInterval(t);
  }, []);

  const add = async () => {
    if (!draft.url) return;
    setBusy(true);
    setErr(null);
    try {
      await api.addWebhookSubscriber({
        id: draft.url,
        url: draft.url,
        secret: draft.secret,
        event_types: draft.event_types.split(",").map((s) => s.trim()).filter(Boolean),
        max_retries: 3,
      });
      setDraft({ url: "", secret: "", event_types: "" });
      await load();
    } catch (e) {
      setErr(e as Error);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    setBusy(true);
    try {
      await api.removeWebhookSubscriber(id);
      await load();
    } catch (e) {
      setErr(e as Error);
    } finally {
      setBusy(false);
    }
  };

  if (err && !subs) return <ErrorBox error={err} />;
  if (!subs || !dlq || !stats) return <Skeleton rows={6} />;

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Webhooks</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Outbound event subscribers.
          </p>
        </header>

        <section className="grid grid-cols-4 gap-4">
          <Stat label="Subscribers" value={stats.subscribers} />
          <Stat label="Delivered" value={stats.delivered} />
          <Stat label="Failed" value={stats.failed} accent="red" />
          <Stat label="DLQ" value={stats.dlq} accent="red" />
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Add subscriber</h2>
          <div className="flex gap-2 flex-wrap">
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[200px]"
              placeholder="https://example.com/hook"
              value={draft.url}
              onChange={(e) => setDraft({ ...draft, url: e.target.value })}
            />
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[200px]"
              placeholder="shared secret (optional)"
              value={draft.secret}
              onChange={(e) => setDraft({ ...draft, secret: e.target.value })}
            />
            <input
              className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm flex-1 min-w-[200px]"
              placeholder="event types, comma separated"
              value={draft.event_types}
              onChange={(e) => setDraft({ ...draft, event_types: e.target.value })}
            />
            <button
              disabled={busy || !draft.url}
              onClick={add}
              className="rounded bg-grafana-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
            >
              Add
            </button>
          </div>
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Subscribers</h2>
          {subs.length === 0 ? (
            <p className="text-sm text-grafana-muted">No subscribers yet.</p>
          ) : (
            <div className="overflow-x-auto rounded border border-grafana-border">
              <table className="w-full text-sm">
                <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                  <tr>
                    <th className="px-3 py-2 text-left">URL</th>
                    <th className="px-3 py-2 text-left">Events</th>
                    <th className="px-3 py-2 text-left">Secret</th>
                    <th className="px-3 py-2 text-right" />
                  </tr>
                </thead>
                <tbody>
                  {subs.map((s) => (
                    <tr key={s.id} className="border-t border-grafana-border">
                      <td className="px-3 py-2 font-mono">{s.url}</td>
                      <td className="px-3 py-2">{s.event_types.join(", ")}</td>
                      <td className="px-3 py-2">{s.secret ? "***" : "—"}</td>
                      <td className="px-3 py-2 text-right">
                        <button
                          disabled={busy}
                          onClick={() => remove(s.id)}
                          className="text-red-400 hover:underline text-xs"
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Dead-letter queue ({dlq.length})</h2>
          {dlq.length === 0 ? (
            <p className="text-sm text-grafana-muted">Nothing in the DLQ.</p>
          ) : (
            <ul className="space-y-2">
              {dlq.map((d, i) => (
                <li
                  key={i}
                  className="rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm"
                >
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-mono">{d.event_id}</span>
                    <span className="text-grafana-muted text-xs">
                      to {d.subscriber_id} status {d.status} attempts {d.attempts}
                    </span>
                  </div>
                  <p className="text-xs text-red-400">{d.error}</p>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </FadeIn>
  );
}

function Stat({ label, value, accent }: { label: string; value: number; accent?: "red" | "green" }) {
  const colour =
    accent === "red"
      ? "text-red-400"
      : accent === "green"
      ? "text-emerald-400"
      : "";
  return (
    <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3">
      <div className="text-xs uppercase text-grafana-muted">{label}</div>
      <div className={"text-2xl font-semibold mt-1 " + colour}>{value}</div>
    </div>
  );
}
