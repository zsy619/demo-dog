// Pulse — a glowing dot whose halo expands & fades on a loop.
//
// Use cases: the green "live" dot on the sidebar, the recording indicator on
// the Live page, an active WS connection chip in the TopBar.

interface Props {
  color?: string;
  size?: number;
  className?: string;
}

export default function Pulse({
  color = "#10b981",
  size = 8,
  className,
}: Props) {
  const halo = size * 2.4;
  return (
    <span
      className={className}
      style={{
        position: "relative",
        display: "inline-block",
        width: size,
        height: size,
      }}
    >
      <span
        style={{
          position: "absolute",
          inset: 0,
          borderRadius: "50%",
          background: color,
          boxShadow: `0 0 ${size}px ${color}`,
        }}
      />
      <span
        style={{
          position: "absolute",
          left: "50%",
          top: "50%",
          width: halo,
          height: halo,
          marginLeft: -halo / 2,
          marginTop: -halo / 2,
          borderRadius: "50%",
          background: color,
          opacity: 0.35,
          animation: "pulse 1.6s ease-out infinite",
        }}
      />
      <style>{`
        @keyframes pulse {
          0%   { transform: scale(0.6); opacity: 0.55; }
          70%  { transform: scale(1.6); opacity: 0; }
          100% { transform: scale(1.6); opacity: 0; }
        }
      `}</style>
    </span>
  );
}
