// GradientText — multi-stop animated gradient sweep across the text. Used
// for the hero metric on the Overview page so the most important number
// "feels alive" without being distracting.
//
// Inspired by react-bits GradientText. Honours prefers-reduced-motion by
// falling back to a static single-colour gradient.

import { useEffect, useState, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  className?: string;
  colors?: string[];
}

const DEFAULTS = ["#a855f7", "#3b82f6", "#10b981", "#a855f7"];

export default function GradientText({
  children,
  className,
  colors = DEFAULTS,
}: Props) {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    if (typeof window === "undefined") return;
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(mq.matches);
    const onChange = () => setReduced(mq.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const grad = colors.join(", ");

  return (
    <span
      className={className}
      style={{
        backgroundImage: `linear-gradient(90deg, ${grad})`,
        backgroundSize: reduced ? "100%" : "200% 100%",
        WebkitBackgroundClip: "text",
        backgroundClip: "text",
        color: "transparent",
        WebkitTextFillColor: "transparent",
        animation: reduced ? undefined : "gradientSlide 8s linear infinite",
      }}
    >
      {children}
      <style>{`
        @keyframes gradientSlide {
          0%   { background-position: 0% 50%; }
          100% { background-position: -200% 50%; }
        }
      `}</style>
    </span>
  );
}
