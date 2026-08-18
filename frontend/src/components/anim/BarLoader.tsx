// BarLoader — three animated vertical bars used in place of a generic
// Skeleton when the content will be a bar/histogram visualisation. Each
// bar grows to a different height on a loop, suggesting "data is being
// resolved" without being decorative.
//
// Use: <BarLoader /> inside a chart placeholder.

interface Props {
  height?: number;
  color?: string;
  className?: string;
}

export default function BarLoader({
  height = 28,
  color = "#3b82f6",
  className,
}: Props) {
  return (
    <span
      className={className}
      style={{
        display: "inline-flex",
        alignItems: "flex-end",
        gap: 4,
        height,
      }}
      aria-label="loading chart"
    >
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          style={{
            display: "inline-block",
            width: 4,
            height: "100%",
            background: color,
            opacity: 0.6,
            transformOrigin: "bottom",
            animation: `barPulse 1s ease-in-out ${i * 0.18}s infinite`,
          }}
        />
      ))}
      <style>{`
        @keyframes barPulse {
          0%, 100% { transform: scaleY(0.35); opacity: 0.4; }
          50%      { transform: scaleY(1);    opacity: 0.95; }
        }
      `}</style>
    </span>
  );
}
