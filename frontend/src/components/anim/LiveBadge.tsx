// LiveBadge — a tiny "REC ●" chip that pulses red. Inspired by recording
// indicators used in video / streaming tools (and by react-bits Aurora).
//
// Use: <LiveBadge label="REC" />

interface Props {
  label?: string;
  className?: string;
}

export default function LiveBadge({ label = "REC", className }: Props) {
  return (
    <span
      className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider border border-grafana-err/40 bg-grafana-err/10 text-grafana-err ${className ?? ""}`}
    >
      <span
        style={{
          display: "inline-block",
          width: 6,
          height: 6,
          borderRadius: "50%",
          background: "#ef4444",
          boxShadow: "0 0 6px #ef4444",
          animation: "recBlink 1.4s ease-in-out infinite",
        }}
        aria-hidden="true"
      />
      {label}
      <style>{`
        @keyframes recBlink {
          0%, 100% { opacity: 1; transform: scale(1); }
          50%      { opacity: 0.35; transform: scale(0.85); }
        }
      `}</style>
    </span>
  );
}
