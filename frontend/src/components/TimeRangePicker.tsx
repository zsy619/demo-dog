import { useState } from "react";

interface Props {
  value: string;
  onChange: (v: string) => void;
}

const PRESETS: Array<{ key: string; label: string; ms: number }> = [
  { key: "5m", label: "Last 5m", ms: 5 * 60 * 1000 },
  { key: "15m", label: "Last 15m", ms: 15 * 60 * 1000 },
  { key: "1h", label: "Last 1h", ms: 60 * 60 * 1000 },
  { key: "6h", label: "Last 6h", ms: 6 * 60 * 60 * 1000 },
  { key: "24h", label: "Last 24h", ms: 24 * 60 * 60 * 1000 },
  { key: "all", label: "All", ms: 0 },
];

export default function TimeRangePicker({ value, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const current = PRESETS.find((p) => p.key === value) ?? PRESETS[0];
  return (
    <div className="relative inline-block text-left">
      <button
        onClick={() => setOpen((v) => !v)}
        className="bg-grafana-elev border border-grafana-border rounded px-2 py-1 text-xs hover:bg-grafana-elev2"
      >
        ⏱ {current.label}
      </button>
      {open && (
        <div className="absolute z-50 mt-1 bg-grafana-elev border border-grafana-border rounded shadow-lg min-w-[140px]">
          {PRESETS.map((p) => (
            <button
              key={p.key}
              onClick={() => {
                onChange(p.key);
                setOpen(false);
              }}
              className={`block w-full text-left px-3 py-1.5 text-xs hover:bg-grafana-elev2 ${
                p.key === value ? "text-grafana-blue" : ""
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
