import { useEffect, useState } from "react";
import { useReplicaStatus } from "@/hooks/queries";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

export default function Replica() {
  const { data, error, isLoading } = useReplicaStatus();
  const [_, setTick] = useState(0);
  useEffect(() => {
    const t = setInterval(() => setTick((x) => x + 1), 5000);
    return () => clearInterval(t);
  }, []);

  if (error) return <ErrorBox error={error as Error} />;
  if (isLoading || !data) return <Skeleton rows={4} />;

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Replica state</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Current replication role and peer offsets.
          </p>
        </header>

        <section className="grid grid-cols-3 gap-4">
          <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3">
            <div className="text-xs uppercase text-grafana-muted">Role</div>
            <div className="text-2xl font-semibold mt-1">{data.role || "standalone"}</div>
          </div>
          <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3">
            <div className="text-xs uppercase text-grafana-muted">Pending</div>
            <div className="text-2xl font-semibold mt-1">{String(data.pending)}</div>
          </div>
          <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3">
            <div className="text-xs uppercase text-grafana-muted">Committed</div>
            <div className="text-2xl font-semibold mt-1">{String(data.committed)}</div>
          </div>
        </section>

        <section>
          <h2 className="text-lg font-medium mb-2">Peers</h2>
          {(!data.peers || data.peers.length === 0) ? (
            <p className="text-sm text-grafana-muted">No peers configured.</p>
          ) : (
            <div className="overflow-x-auto rounded border border-grafana-border">
              <table className="w-full text-sm">
                <thead className="bg-grafana-panel-2 text-xs uppercase text-grafana-muted">
                  <tr>
                    <th className="px-3 py-2 text-left">ID</th>
                    <th className="px-3 py-2 text-right">Last offset</th>
                    <th className="px-3 py-2 text-right">Last ack</th>
                  </tr>
                </thead>
                <tbody>
                  {data.peers.map((p) => (
                    <tr key={p.id} className="border-t border-grafana-border">
                      <td className="px-3 py-2 font-mono">{p.id}</td>
                      <td className="px-3 py-2 text-right">{p.last_offset}</td>
                      <td className="px-3 py-2 text-right">{p.last_ack}</td>
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
