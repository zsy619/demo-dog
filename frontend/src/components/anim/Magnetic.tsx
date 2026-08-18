// Magnetic — gentle pull-to-cursor transform on hover. Used sparingly on
// hero tiles so the page feels alive without every card competing for
// attention.
//
// Implementation: a mousemove handler translates the wrapper toward the
// cursor by up to `strength` px. Mouse-out springs it back. Touch devices
// never fire mousemove → no jitter. Reduced-motion users skip the transform.

import { useEffect, useRef, useState, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  className?: string;
  /** Maximum pixel offset toward the cursor. Default 6. */
  strength?: number;
}

export default function Magnetic({
  children,
  className,
  strength = 6,
}: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [transform, setTransform] = useState("translate(0px, 0px)");
  const [enabled, setEnabled] = useState(true);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setEnabled(!mq.matches);
    const onChange = () => setEnabled(!mq.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const handleMove = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!enabled || !ref.current) return;
    const rect = ref.current.getBoundingClientRect();
    const cx = rect.left + rect.width / 2;
    const cy = rect.top + rect.height / 2;
    const dx = ((e.clientX - cx) / (rect.width / 2)) * strength;
    const dy = ((e.clientY - cy) / (rect.height / 2)) * strength;
    setTransform(
      `translate(${dx.toFixed(2)}px, ${dy.toFixed(2)}px)`
    );
  };

  const handleLeave = () => setTransform("translate(0px, 0px)");

  return (
    <div
      ref={ref}
      className={className}
      onMouseMove={handleMove}
      onMouseLeave={handleLeave}
      style={{
        transition: "transform 220ms cubic-bezier(0.22, 1, 0.36, 1)",
        transform,
      }}
    >
      {children}
    </div>
  );
}
