// Stagger — wraps each direct child in a FadeIn with a per-index delay so
// a freshly-rendered list "cascades" in row by row instead of popping.
//
// Cap at ~24 children so a 500-row log table does not queue 500 delayed
// keyframes. Children past that limit render immediately.

import type { ReactNode } from "react";
import FadeIn from "./FadeIn";

interface Props {
  children: ReactNode[];
  step?: number; // ms between siblings
  initial?: number;
  max?: number;
}

export default function Stagger({
  children,
  step = 30,
  initial = 0,
  max = 24,
}: Props) {
  return (
    <>
      {children.map((child, i) =>
        i < max ? (
          <FadeIn key={i} delay={i * step + initial} duration={220}>
            {child}
          </FadeIn>
        ) : (
          <div key={i}>{child}</div>
        )
      )}
    </>
  );
}
