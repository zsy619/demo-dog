// Glitch — a brief RGB-split + skew shake animation.
//
// Reserved for high-severity indicators (error rate > 5%, FATAL logs, hard
// outage). Plays once on mount; callers that want it to repeat on data change
// can remount via a key={value} prop.
//
// Use sparingly. Glitch is loud; the dashboard already uses colour to signal
// severity, so an animation here is the second channel, not the first.

import type { ReactNode } from "react";

interface Props {
  children: ReactNode;
  className?: string;
  duration?: number;
}

export default function Glitch({ children, className, duration = 400 }: Props) {
  return (
    <span className={className} style={{ position: "relative", display: "inline-block" }}>
      <span style={{ position: "relative" }}>{children}</span>
      <span
        aria-hidden="true"
        style={{
          position: "absolute",
          inset: 0,
          color: "#ef4444",
          mixBlendMode: "screen",
          animation: `glitchRgb ${duration}ms steps(6) 1`,
          pointerEvents: "none",
        }}
      >
        {children}
      </span>
      <style>{`
        @keyframes glitchRgb {
          0%   { transform: translate(0,0); }
          20%  { transform: translate(-2px,1px); }
          40%  { transform: translate(2px,-1px); }
          60%  { transform: translate(-1px,2px); }
          80%  { transform: translate(1px,-1px); }
          100% { transform: translate(0,0); }
        }
      `}</style>
    </span>
  );
}
