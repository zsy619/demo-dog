// FadeIn — wraps any child and fades it in on mount, optionally up to a
// per-row stagger delay.
//
// Useful for: a freshly-loaded table of logs, a dashboard panel that just
// resolved its query, a toast appearing in the corner.

import type { ReactNode } from "react";

interface Props {
  children: ReactNode;
  delay?: number;
  duration?: number;
  /** Where the element starts on the Y axis. Default 6px. */
  fromY?: number;
  className?: string;
}

export default function FadeIn({
  children,
  delay = 0,
  duration = 250,
  fromY = 6,
  className,
}: Props) {
  return (
    <div
      className={className}
      style={{
        opacity: 0,
        transform: `translateY(${fromY}px)`,
        animation: `fadeIn ${duration}ms ease-out ${delay}ms forwards`,
      }}
    >
      {children}
      <style>{`
        @keyframes fadeIn {
          to {
            opacity: 1;
            transform: translateY(0);
          }
        }
      `}</style>
    </div>
  );
}
