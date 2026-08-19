import { useState } from "react";
import { api } from "@/lib/api";
import type { ProbeResult } from "@/types/api";
import { ErrorBox } from "@/components/Feedback";
import FadeIn from "@/components/anim/FadeIn";

export default function Probes() {
  const [target, setTarget] = useState("");
  const [result, setResult] = useState<ProbeResult | null>(null);
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(false);

  const run = async () => {
    if (!target) return;
    setLoading(true);
    setErr(null);
    try {
      const r = await api.probe(target);
      setResult(r);
    } catch (e) {
      setErr(e as Error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <FadeIn>
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Probes</h1>
          <p className="text-sm text-grafana-muted mt-1">
            Run a one-shot HTTP probe through the collector.
          </p>
        </header>

        <section className="flex gap-2">
          <input
            type="url"
            placeholder="https://example.com/health"
            className="flex-1 rounded border border-grafana-border bg-grafana-panel-2 px-3 py-2 text-sm"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
          />
          <button
            onClick={run}
            disabled={loading || !target}
            className="rounded bg-grafana-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
          >
            {loading ? "Probing..." : "Run"}
          </button>
        </section>

        {err && <ErrorBox error={err} />}

        {result && (
          <section>
            <h2 className="text-lg font-medium mb-2">Result</h2>
            <div className="rounded border border-grafana-border bg-grafana-panel-2 p-3 text-sm">
              <div>Target: <span className="font-mono">{result.target}</span></div>
              <div>
                Status: <span className={result.ok ? "text-emerald-400" : "text-red-400"}>{result.status_code}</span>
              </div>
              <div>Duration: {(result.duration_ns / 1e6).toFixed(2)}ms</div>
            </div>
          </section>
        )}
      </div>
    </FadeIn>
  );
}
