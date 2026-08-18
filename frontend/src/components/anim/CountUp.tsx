// CountUp — ease-out roll from previous value to new value.
//
// Why: in an observability dashboard a QPS/p99 tile that "snaps" to its new
// value hides the *fact that something changed*. Counting from the old value
// surfaces the change visually without adding layout shift.
//
// Implementation notes:
//   - Uses requestAnimationFrame for a 60fps feel.
//   - Easing: cubic ease-out so the new value "settles".
//   - Honours `reduced motion` (skips the animation entirely).
//   - Formatter is caller-supplied so we can keep the comma-separated look.

import { useEffect, useRef, useState } from "react";

interface Props {
  value: number;
  duration?: number;
  /** Override the displayed number (e.g. for ms / s units). Defaults to Math.round. */
  format?: (n: number) => string;
  className?: string;
}

function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3);
}

export default function CountUp({
  value,
  duration = 600,
  format,
  className,
}: Props) {
  const [display, setDisplay] = useState(value);
  const fromRef = useRef(value);
  const startRef = useRef<number | null>(null);
  const rafRef = useRef<number | null>(null);

  useEffect(() => {
    if (
      typeof window !== "undefined" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches
    ) {
      setDisplay(value);
      fromRef.current = value;
      return;
    }
    fromRef.current = display;
    startRef.current = null;

    const tick = (ts: number) => {
      if (startRef.current === null) startRef.current = ts;
      const elapsed = ts - startRef.current;
      const t = Math.min(1, elapsed / duration);
      const eased = easeOutCubic(t);
      const next = fromRef.current + (value - fromRef.current) * eased;
      setDisplay(next);
      if (t < 1) {
        rafRef.current = requestAnimationFrame(tick);
      } else {
        setDisplay(value);
        rafRef.current = null;
      }
    };

    if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(tick);
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, duration]);

  const text = format ? format(display) : Math.round(display).toString();
  return <span className={className}>{text}</span>;
}
