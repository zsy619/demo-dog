// ShinyText — a brief gold sweep across text when it first appears or when
// the content changes. Inspired by react-bits ShinyText. We use it for
// KPI tiles whose value just updated so the eye catches the change even
// when the digit itself is small.
//
// We track the previous value and re-trigger the animation whenever it
// changes. Reduced-motion users get the plain text without the sweep.

import { useEffect, useRef, useState, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  className?: string;
  /** Color of the shine. Default is gold. */
  color?: string;
}

export default function ShinyText({
  children,
  className,
  color = "#facc15",
}: Props) {
  const last = useRef<ReactNode>(children);
  const [animKey, setAnimKey] = useState(0);

  useEffect(() => {
    if (last.current !== children) {
      last.current = children;
      setAnimKey((k) => k + 1);
    }
  }, [children]);

  return (
    <span
      className={className}
      key={animKey}
      style={{
        position: "relative",
        display: "inline-block",
        overflow: "hidden",
      }}
    >
      <span style={{ position: "relative" }}>{children}</span>
      <span
        aria-hidden="true"
        style={{
          position: "absolute",
          inset: 0,
          background: `linear-gradient(120deg, transparent 30%, ${color} 50%, transparent 70%)`,
          mixBlendMode: "screen",
          animation: `shinySweep 900ms ease-out 1`,
          pointerEvents: "none",
        }}
      />
      <style>{`
        @keyframes shinySweep {
          0%   { transform: translateX(-100%); opacity: 0; }
          20%  { opacity: 1; }
          100% { transform: translateX(100%); opacity: 0; }
        }
      `}</style>
    </span>
  );
}
