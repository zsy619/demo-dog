// TrueFocus — soft animated halo behind text, used on alert titles.
//
// A glowing rectangle behind the text gently shifts position to suggest
// "this is being attended to". Loops at 3s so it is unobtrusive but always
// visible while the alert is on screen.

import type { ReactNode } from "react";

interface Props {
  children: ReactNode;
  color?: string;
  className?: string;
}

export default function TrueFocus({
  children,
  color = "#ef4444",
  className,
}: Props) {
  return (
    <span
      className={className}
      style={{ position: "relative", display: "inline-block", padding: "2px 6px" }}
    >
      <span
        aria-hidden="true"
        style={{
          position: "absolute",
          inset: 0,
          background: color,
          opacity: 0.18,
          borderRadius: 4,
          filter: "blur(6px)",
          animation: "focusPulse 3s ease-in-out infinite",
        }}
      />
      <span style={{ position: "relative" }}>{children}</span>
      <style>{`
        @keyframes focusPulse {
          0%, 100% { opacity: 0.12; transform: scale(1); }
          50%      { opacity: 0.28; transform: scale(1.04); }
        }
      `}</style>
    </span>
  );
}
