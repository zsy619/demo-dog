import type { Severity } from "@/types/api";

const COLORS: Record<Severity, string> = {
  TRACE: "bg-grafana-elev text-grafana-muted",
  DEBUG: "bg-grafana-elev text-grafana-muted",
  INFO: "bg-blue-500/15 text-blue-300",
  WARN: "bg-amber-500/15 text-amber-300",
  ERROR: "bg-red-500/15 text-red-300",
  FATAL: "bg-red-700/30 text-red-200",
};

// Map alternate spellings that the OTLP world throws at us
// (warning/err/panic/emerg/etc.) onto the canonical 6 levels we display.
function normalize(s: string): Severity {
  const u = (s || "").toUpperCase().trim();
  if (u === "WARN" || u === "WARNING" || u === "NOTICE") return "WARN";
  if (u === "ERR" || u === "ERROR" || u === "CRITICAL") return "ERROR";
  if (u === "FATAL" || u === "PANIC" || u === "EMERG" || u === "EMERGENCY" || u === "ALERT")
    return "FATAL";
  if (u === "INFO" || u === "INFORMATION" || u === "DEBUG" || u === "TRACE")
    return u as Severity;
  return "INFO";
}

export default function SeverityBadge({
  value,
  className = "",
}: {
  value: string;
  className?: string;
}) {
  const sev = normalize(value);
  const cls = COLORS[sev];
  return (
    <span
      className={`inline-block px-1.5 py-0.5 rounded-sm text-[10px] font-mono uppercase tracking-wider ${cls} ${className}`}
      title={`raw: ${value}`}
    >
      {sev}
    </span>
  );
}
