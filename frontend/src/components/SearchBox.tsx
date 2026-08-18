import { useEffect, useRef, useState } from "react";

interface Props {
  value: string;
  onChange: (v: string) => void;
  onSubmit?: (v: string) => void;
  placeholder?: string;
  /** Debounce delay in ms; 0 means immediate. */
  debounce?: number;
  className?: string;
}

// SearchBox wraps a styled <input> with debouncing and a clear button.
export default function SearchBox({
  value,
  onChange,
  onSubmit,
  placeholder,
  debounce = 250,
  className = "",
}: Props) {
  const [local, setLocal] = useState(value);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => setLocal(value), [value]);

  useEffect(() => {
    if (debounce <= 0) {
      onChange(local);
      return;
    }
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      onChange(local);
    }, debounce);
    return () => {
      if (timer.current) window.clearTimeout(timer.current);
    };
  }, [local, debounce, onChange]);

  return (
    <div className={`relative ${className}`}>
      <svg
        className="absolute left-2 top-1/2 -translate-y-1/2 w-4 h-4 text-grafana-text-secondary pointer-events-none"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
      >
        <circle cx="11" cy="11" r="7" />
        <path d="m20 20-3-3" />
      </svg>
      <input
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") onSubmit?.(local);
        }}
        placeholder={placeholder ?? "Search…"}
        className="w-full bg-grafana-elev border border-grafana-border rounded pl-8 pr-8 py-1.5 text-sm font-mono focus:outline-none focus:border-grafana-blue"
      />
      {local && (
        <button
          aria-label="Clear search"
          className="absolute right-2 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-grafana-text-secondary hover:text-grafana-text"
          onClick={() => {
            setLocal("");
            onChange("");
          }}
        >
          ×
        </button>
      )}
    </div>
  );
}
