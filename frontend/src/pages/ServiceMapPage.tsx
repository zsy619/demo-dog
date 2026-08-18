import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { ServiceMap, ServiceSummary } from "@/types/api";
import { ErrorBox, Skeleton } from "@/components/Feedback";
import { duration } from "@/lib/time";

export default function ServiceMapPage() {
  const [map, setMap] = useState<ServiceMap | null>(null);
  const [services, setServices] = useState<ServiceSummary[]>([]);
  const [err, setErr] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);
  const [hovered, setHovered] = useState<string | null>(null);

  const drillInto = (name: string) => {
    if (typeof window !== "undefined") {
      window.location.hash = `#/service-detail?service=${encodeURIComponent(name)}`;
    }
  };

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [m, svcs] = await Promise.all([api.serviceMap(), api.services()]);
        if (cancelled) return;
        setMap(m);
        setServices(svcs.services);
        setErr(null);
      } catch (e) {
        if (!cancelled) setErr(e as Error);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    const id = window.setInterval(load, 8000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  const nodeQPS = (name: string) => services.find((s) => s.name === name)?.qps ?? 0;
  const nodeErr = (name: string) => services.find((s) => s.name === name)?.error_rate ?? 0;

  return (
    <div className="p-6 space-y-4">
      {err && <ErrorBox error={err} />}
      <div>
        <div className="text-[13px] font-semibold">Service dependency graph</div>
        <div className="text-[11px] text-grafana-muted">
          Caller → callee edges derived from span parents. Node size scales with QPS; red intensity tracks error rate.
        </div>
      </div>

      {loading ? (
        <Skeleton rows={4} />
      ) : !map || map.nodes.length === 0 ? (
        <div className="text-grafana-muted italic text-center py-12">
          No edges yet. Run a seed or open the Ingest demo.
        </div>
      ) : (
        <ServiceMapGraph
          map={map}
          hovered={hovered}
          setHovered={setHovered}
          nodeQPS={nodeQPS}
          nodeErr={nodeErr}
          onNodeClick={drillInto}
        />
      )}

      <div className="bg-grafana-panel border border-grafana-border rounded-lg overflow-hidden">
        <div className="px-4 py-2 border-b border-grafana-border text-[12px] text-grafana-muted">
          Edges ({map?.edges.length ?? 0})
        </div>
        <table className="w-full text-sm">
          <thead className="bg-grafana-elev text-[11px] text-grafana-muted uppercase tracking-wider">
            <tr>
              <th className="px-3 py-2 text-left">Caller</th>
              <th className="px-3 py-2 text-left">Callee</th>
              <th className="px-3 py-2 text-right">Calls</th>
              <th className="px-3 py-2 text-right">Errors</th>
              <th className="px-3 py-2 text-right">Err rate</th>
              <th className="px-3 py-2 text-right">avg</th>
              <th className="px-3 py-2 text-right">p99</th>
            </tr>
          </thead>
          <tbody>
            {(map?.edges ?? []).map((e, i) => (
              <tr
                key={i}
                className="border-t border-grafana-border hover:bg-grafana-elev/50"
              >
                <td className="px-3 py-1.5 text-grafana-accent">{e.from}</td>
                <td className="px-3 py-1.5 text-grafana-blue">{e.to}</td>
                <td className="px-3 py-1.5 text-right font-mono">{e.calls}</td>
                <td className="px-3 py-1.5 text-right font-mono">
                  <span className={e.errors > 0 ? "text-grafana-err" : ""}>{e.errors}</span>
                </td>
                <td className="px-3 py-1.5 text-right">
                  <span
                    className={
                      e.errors / Math.max(1, e.calls) > 0.05
                        ? "text-grafana-err"
                        : "text-grafana-muted"
                    }
                  >
                    {((e.errors / Math.max(1, e.calls)) * 100).toFixed(1)}%
                  </span>
                </td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {duration(e.avg_ms)}
                </td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {duration(e.p99_ms)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function ServiceMapGraph({
  map,
  hovered,
  setHovered,
  nodeQPS,
  nodeErr,
  onNodeClick,
}: {
  map: ServiceMap;
  hovered: string | null;
  setHovered: (n: string | null) => void;
  nodeQPS: (n: string) => number;
  nodeErr: (n: string) => number;
  onNodeClick: (n: string) => void;
}) {
  // Layout: place nodes on a circle for a stable look. Edge curvature is
  // computed analytically using midpoint + perpendicular offset.
  const W = 720;
  const H = 480;
  const cx = W / 2;
  const cy = H / 2;
  const radius = 170;
  const positions: Record<string, { x: number; y: number }> = {};
  const nodes = map.nodes;
  nodes.forEach((name, i) => {
    const angle = (2 * Math.PI * i) / Math.max(1, nodes.length) - Math.PI / 2;
    positions[name] = {
      x: cx + radius * Math.cos(angle),
      y: cy + radius * Math.sin(angle),
    };
  });

  const maxQPS = Math.max(1, ...nodes.map((n) => nodeQPS(n)));

  return (
    <div className="bg-grafana-panel border border-grafana-border rounded-lg p-4 overflow-auto">
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-auto max-w-full">
        <defs>
          <marker
            id="arrow"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerUnits="strokeWidth"
            markerWidth="6"
            markerHeight="6"
            orient="auto"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#64748b" />
          </marker>
          <marker
            id="arrow-err"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerUnits="strokeWidth"
            markerWidth="6"
            markerHeight="6"
            orient="auto"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#ef4444" />
          </marker>
        </defs>
        {map.edges.map((e, i) => {
          const a = positions[e.from];
          const b = positions[e.to];
          if (!a || !b) return null;
          const dx = b.x - a.x;
          const dy = b.y - a.y;
          const mx = (a.x + b.x) / 2;
          const my = (a.y + b.y) / 2;
          const len = Math.hypot(dx, dy) || 1;
          const off = len * 0.15;
          const cpx = mx + (-dy / len) * off;
          const cpy = my + (dx / len) * off;
          const isErr = e.errors > 0;
          const isHot = hovered === e.from || hovered === e.to;
          return (
            <path
              key={i}
              d={`M ${a.x} ${a.y} Q ${cpx} ${cpy} ${b.x} ${b.y}`}
              stroke={isErr ? "#ef4444" : "#64748b"}
              strokeOpacity={isHot ? 0.9 : 0.35}
              strokeWidth={1 + Math.min(4, e.calls / 4)}
              fill="none"
              markerEnd={isErr ? "url(#arrow-err)" : "url(#arrow)"}
              onMouseEnter={() => setHovered(e.from)}
              onMouseLeave={() => setHovered(null)}
            >
              <title>
                {e.from} → {e.to}: {e.calls} calls, {e.errors} errors, p99 {duration(e.p99_ms)}
              </title>
            </path>
          );
        })}
        {nodes.map((name) => {
          const p = positions[name];
          if (!p) return null;
          const qps = nodeQPS(name);
          const err = nodeErr(name);
          const r = 14 + (qps / maxQPS) * 20;
          const fill =
            err > 0.1
              ? "#7f1d1d"
              : err > 0.05
              ? "#9a3412"
              : "#1e3a8a";
          const isHot = hovered === name;
          return (
            <g
              key={name}
              onMouseEnter={() => setHovered(name)}
              onMouseLeave={() => setHovered(null)}
              onClick={() => onNodeClick(name)}
              style={{ cursor: "pointer" }}
            >
              <circle
                cx={p.x}
                cy={p.y}
                r={r}
                fill={fill}
                fillOpacity={isHot ? 0.9 : 0.65}
                stroke={isHot ? "#fff" : "#cbd5e1"}
                strokeWidth={isHot ? 2 : 1}
              />
              <text
                x={p.x}
                y={p.y + r + 12}
                textAnchor="middle"
                fontSize={11}
                fontFamily="ui-monospace, monospace"
                fill="#cbd5e1"
              >
                {name}
              </text>
              <text
                x={p.x}
                y={p.y + 3}
                textAnchor="middle"
                fontSize={10}
                fill="#fff"
              >
                {qps.toFixed(1)}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}
