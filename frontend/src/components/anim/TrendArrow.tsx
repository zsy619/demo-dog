// TrendArrow — animated ↑/↓ arrow that bounces when direction changes.
//
// Observability dashboards rely on deltas to communicate "things got
// better/worse". A static ↑ looks identical to a static ↓ if you blink,
// so we add a brief up/down keyframe when the direction flips.
//
// Use: <TrendArrow delta={x} suffix="%" />

import { useEffect, useRef, useState } from "react";

interface Props {
  delta: number;
  suffix?: string;
  className?: string;
}

export default function TrendArrow({ delta, suffix = "%", className }: Props) {
  const lastSignRef = useRef<number>(Math.sign(delta));
  const [animKey, setAnimKey] = useState(0);
  const sign = Math.sign(delta);

  useEffect(() => {
    if (sign !== 0 && sign !== lastSignRef.current) {
      lastSignRef.current = sign;
      setAnimKey((k) => k + 1);
    } else if (sign === 0) {
      lastSignRef.current = 0;
    }
  }, [sign]);

  if (sign === 0) {
    return (
      <span className={className} key={`flat-${animKey}`}>
        <span aria-hidden="true">·</span>
        <span className="font-mono tabular-nums">
          {Math.abs(delta).toFixed(2)}
          {suffix}
        </span>
      </span>
    );
  }

  const arrow = sign > 0 ? "↑" : "↓";
  const animName = sign > 0 ? "bounceUp" : "bounceDown";
  const colorClass = sign > 0 ? "text-grafana-ok" : "text-grafana-err";

  return (
    <span className={`${colorClass} ${className ?? ""}`} key={animKey}>
      <span
        style={{
          display: "inline-block",
          animation: `${animName} 380ms cubic-bezier(0.22, 1, 0.36, 1) 1`,
        }}
        aria-hidden="true"
      >
        {arrow}
      </span>
      <span className="font-mono tabular-nums">
        {Math.abs(delta).toFixed(2)}
        {suffix}
      </span>
      <style>{`
        @keyframes bounceUp {
          0%   { transform: translateY(6px); opacity: 0; }
          50%  { transform: translateY(-3px); }
          100% { transform: translateY(0); opacity: 1; }
        }
        @keyframes bounceDown {
          0%   { transform: translateY(-6px); opacity: 0; }
          50%  { transform: translateY(3px); }
          100% { transform: translateY(0); opacity: 1; }
        }
      `}</style>
    </span>
  );
}
