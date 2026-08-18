// Tiny, dependency-free line chart for time-series. SVG-based for crispness.
//
// The chart uses a single path with normalized coordinates. It does not enforce
// minimum heights or anti-alias anything fancy; this is a demo dashboard, not
// a production graphing library.

import { useMemo } from "react";

interface Series {
  name: string;
  points: { ts: number; value: number }[];
  color?: string;
  unit?: string;
}

interface Props {
  series: Series[];
  height?: number;
  width?: number;
  showAxis?: boolean;
  showLegend?: boolean;
  formatValue?: (v: number) => string;
  /** Hide series whose max value is 0 (useful for empty metric channels). */
  hideEmpty?: boolean;
}

const DEFAULT_COLORS = [
  "#3b82f6",
  "#7e3bf2",
  "#1a7f37",
  "#bf8700",
  "#cf222e",
  "#06b6d4",
];

export default function TimeSeriesChart({
  series,
  height = 220,
  width,
  showAxis = true,
  showLegend = false,
  formatValue,
  hideEmpty = false,
}: Props) {
  const W = width ?? 600;
  const H = height;
  const PAD = showAxis ? { l: 50, r: 12, t: 16, b: 24 } : { l: 0, r: 0, t: 0, b: 0 };

  const { allPoints, xMin, xMax, yMin, yMax } = useMemo(() => {
    // Defensive: the backend occasionally returns `null` for individual
    // points (e.g. when a bucket has zero samples) or for the whole series
    // when no data exists. Filter to well-formed points so downstream code
    // can assume `p.value` and `p.ts` are numbers.
    const all: { ts: number; value: number }[] = [];
    for (const s of series) {
      if (!s || !s.points) continue;
      for (const p of s.points) {
        if (
          p !== null &&
          p !== undefined &&
          typeof p.ts === "number" &&
          typeof p.value === "number" &&
          Number.isFinite(p.ts) &&
          Number.isFinite(p.value)
        ) {
          all.push(p);
        }
      }
    }
    if (all.length === 0) {
      return {
        allPoints: all,
        xMin: 0,
        xMax: 1,
        yMin: 0,
        yMax: 1,
      };
    }
    const xs = all.map((p) => p.ts);
    const ys = all.map((p) => p.value);
    const xMin = Math.min(...xs);
    const xMax = Math.max(...xs);
    let yMin = Math.min(...ys);
    let yMax = Math.max(...ys);
    if (yMin === yMax) {
      yMin -= 1;
      yMax += 1;
    } else {
      const pad = (yMax - yMin) * 0.1;
      yMin -= pad;
      yMax += pad;
    }
    return { allPoints: all, xMin, xMax, yMin, yMax };
  }, [series]);

  const xScale = (x: number) =>
    PAD.l + ((x - xMin) / (xMax - xMin || 1)) * (W - PAD.l - PAD.r);
  const yScale = (y: number) =>
    PAD.t + (1 - (y - yMin) / (yMax - yMin || 1)) * (H - PAD.t - PAD.b);

  const fmt = formatValue ?? ((v: number) => v.toFixed(1));
  const fmtTime = (ms: number) => {
    const d = new Date(ms);
    return `${d.getHours().toString().padStart(2, "0")}:${d
      .getMinutes()
      .toString()
      .padStart(2, "0")}`;
  };

  const xTicks = useMemo(() => {
    const out: number[] = [];
    const n = 5;
    for (let i = 0; i <= n; i++) {
      out.push(xMin + ((xMax - xMin) * i) / n);
    }
    return out;
  }, [xMin, xMax]);

  const yTicks = useMemo(() => {
    const out: number[] = [];
    const n = 4;
    for (let i = 0; i <= n; i++) {
      out.push(yMin + ((yMax - yMin) * i) / n);
    }
    return out;
  }, [yMin, yMax]);

  // Filter out empty series when requested (max value 0 → no real activity).
  // Defensive against missing points or null entries — see the comment on
  // `allPoints` above for the rationale.
  const visibleSeries = hideEmpty
    ? series.filter((s) => {
        if (!s || !s.points || s.points.length === 0) return false;
        const valid = s.points.filter(
          (p) => p && typeof p.value === "number" && Number.isFinite(p.value)
        );
        if (valid.length === 0) return false;
        return Math.max(...valid.map((p) => p.value)) > 0;
      })
    : series;

  const effectiveAll = (() => {
    if (!hideEmpty) return allPoints;
    const out: { ts: number; value: number }[] = [];
    for (const s of visibleSeries) for (const p of s.points) out.push(p);
    return out;
  })();

  const effectiveYRange = (() => {
    if (!hideEmpty) return { yMin, yMax };
    if (effectiveAll.length === 0) return { yMin: 0, yMax: 1 };
    const ys = effectiveAll.map((p) => p.value);
    let yMi = Math.min(...ys);
    let yMa = Math.max(...ys);
    if (yMi === yMa) {
      yMi -= 1;
      yMa += 1;
    } else {
      const pad = (yMa - yMi) * 0.1;
      yMi -= pad;
      yMa += pad;
    }
    return { yMin: yMi, yMax: yMa };
  })();

  const yScale2 = (y: number) =>
    PAD.t +
    (1 - (y - effectiveYRange.yMin) / (effectiveYRange.yMax - effectiveYRange.yMin || 1)) *
      (H - PAD.t - PAD.b);

  // Alias for clarity: when hideEmpty is true we use the recomputed scale;
  // otherwise we keep the original yScale unchanged.
  const yS = hideEmpty ? yScale2 : yScale;

  const svgChart = (
    <svg
      width={W}
      height={H}
      viewBox={`0 0 ${W} ${H}`}
      className="w-full"
      style={{ maxWidth: "100%" }}
    >
      <rect width={W} height={H} fill="transparent" />
      {showAxis && (
        <g>
          {yTicks.map((t, i) => {
            const y = yScale(t);
            return (
              <g key={`y-${i}`}>
                <line
                  x1={PAD.l}
                  x2={W - PAD.r}
                  y1={y}
                  y2={y}
                  stroke="#22272e"
                  strokeWidth={1}
                />
                <text
                  x={PAD.l - 6}
                  y={y + 4}
                  fontSize={10}
                  fill="#5a6172"
                  textAnchor="end"
                  fontFamily="JetBrains Mono, ui-monospace, monospace"
                >
                  {fmt(t)}
                </text>
              </g>
            );
          })}
          {xTicks.map((t, i) => {
            const x = xScale(t);
            return (
              <text
                key={`x-${i}`}
                x={x}
                y={H - 6}
                fontSize={10}
                fill="#5a6172"
                textAnchor="middle"
                fontFamily="JetBrains Mono, ui-monospace, monospace"
              >
                {fmtTime(t)}
              </text>
            );
          })}
        </g>
      )}

      {allPoints.length === 0 && (
        <text
          x={W / 2}
          y={H / 2}
          fontSize={12}
          fill="#5a6172"
          textAnchor="middle"
        >
          no data
        </text>
      )}

      {visibleSeries.map((s, idx) => {
        const color = s.color ?? DEFAULT_COLORS[idx % DEFAULT_COLORS.length];
        if (!s.points || s.points.length === 0) return null;
        // Skip individual points that are null or have non-numeric values
        // so a malformed bucket doesn’t crash the entire chart.
        const cleanPoints = s.points.filter(
          (p) =>
            p && typeof p.ts === "number" && typeof p.value === "number"
        );
        if (cleanPoints.length === 0) return null;
        const path = cleanPoints
          .map((p, i) => {
            const x = xScale(p.ts);
            const y = yS(p.value);
            return `${i === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
          })
          .join(" ");
        return (
          <g key={s.name}>
            <path d={path} stroke={color} strokeWidth={1.5} fill="none" />
            {cleanPoints.slice(-1).map((p, i) => (
              <circle
                key={`pt-${i}`}
                cx={xScale(p.ts)}
                cy={yS(p.value)}
                r={3}
                fill={color}
              />
            ))}
          </g>
        );
      })}
    </svg>
  );

  if (!showLegend) return svgChart;

  return (
    <div className="flex flex-col gap-1">
      {svgChart}
      <div className="flex items-center gap-2 flex-wrap text-[10px] text-grafana-muted">
        {visibleSeries.map((s, idx) => {
          const color = s.color ?? DEFAULT_COLORS[idx % DEFAULT_COLORS.length];
          // Filter to well-formed points so the legend math never sees null.
          const valid =
            s && s.points
              ? s.points.filter(
                  (p) =>
                    p && typeof p.value === "number" && Number.isFinite(p.value)
                )
              : [];
          const last = valid[valid.length - 1];
          const min = valid.length
            ? Math.min(...valid.map((p) => p.value))
            : 0;
          const max = valid.length
            ? Math.max(...valid.map((p) => p.value))
            : 0;
          return (
            <span
              key={s.name}
              className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded border border-grafana-border bg-grafana-elev/40"
              title={`min ${min.toFixed(2)} · max ${max.toFixed(2)}`}
            >
              <span
                className="inline-block w-2 h-2 rounded-full"
                style={{ background: color }}
              />
              <span className="text-grafana-text">{s.name}</span>
              {last && (
                <span className="font-mono tabular-nums text-grafana-muted">
                  {last.value.toFixed(1)}
                  {s.unit ? ` ${s.unit}` : ""}
                </span>
              )}
            </span>
          );
        })}
        {visibleSeries.length === 0 && (
          <span className="text-grafana-muted italic">no data</span>
        )}
      </div>
    </div>
  );
}
