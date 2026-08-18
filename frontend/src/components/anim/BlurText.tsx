// BlurText — per-character blur-in reveal.
//
// Each glyph starts blurred (filter: blur(6px), translateY(8px)) and animates
// to its final position over `duration` ms, staggered by `stagger` ms. The
// effect is "this title just appeared" without taking up much screen real
// estate.
//
// Inspired by react-bits BlurText, but without any deps. Uses CSS animations
// via inline keyframes so we do not have to add Tailwind animation classes.

import { useEffect, useState } from "react";

interface Props {
  text: string;
  className?: string;
  stagger?: number; // ms between characters
  duration?: number; // per-character animation length
}

export default function BlurText({
  text,
  className,
  stagger = 30,
  duration = 500,
}: Props) {
  // We only animate when the text changes — otherwise the re-render is free.
  const [shown, setShown] = useState(text);
  const [animationKey, setAnimationKey] = useState(0);

  useEffect(() => {
    if (text !== shown) {
      setShown(text);
      setAnimationKey((k) => k + 1);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  return (
    <span className={className} key={animationKey}>
      {Array.from(shown).map((ch, i) => (
        <span
          key={i}
          style={{
            display: "inline-block",
            opacity: 0,
            filter: "blur(6px)",
            transform: "translateY(8px)",
            animation: `blurIn ${duration}ms cubic-bezier(0.22, 1, 0.36, 1) ${i * stagger}ms forwards`,
            whiteSpace: ch === " " ? "pre" : undefined,
          }}
        >
          {ch === " " ? "\u00A0" : ch}
        </span>
      ))}
      <style>{`
        @keyframes blurIn {
          to {
            opacity: 1;
            filter: blur(0px);
            transform: translateY(0);
          }
        }
      `}</style>
    </span>
  );
}
