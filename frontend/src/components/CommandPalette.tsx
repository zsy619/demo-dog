// Command palette overlay (Cmd/Ctrl+K). Provides fuzzy navigation across
// every page, every known service, and common API endpoints.

import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { ServiceSummary } from "@/types/api";
import type { Page } from "@/App";

interface PaletteItem {
  id: string;
  label: string;
  hint?: string;
  section: string;
  perform: () => void;
}

interface Props {
  page: Page;
  navigate: (page: Page, params?: URLSearchParams) => void;
  onServiceChange: (s: string) => void;
}

export default function CommandPalette({ page, navigate, onServiceChange }: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const [services, setServices] = useState<ServiceSummary[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  // Cmd/Ctrl+K opens the palette; Escape closes it.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const cmdK = (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k";
      if (cmdK) {
        e.preventDefault();
        setOpen((v) => !v);
        return;
      }
      if (open && e.key === "Escape") {
        setOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  // Pull service list lazily so the palette has up-to-date data.
  useEffect(() => {
    if (!open) return;
    api
      .services()
      .then((r) => setServices(r.services))
      .catch(() => {});
    setQuery("");
    setCursor(0);
    // focus input after mount
    setTimeout(() => inputRef.current?.focus(), 30);
  }, [open]);

  const items = useMemo<PaletteItem[]>(() => {
    const basePages: Array<{ id: Page; label: string; hint: string }> = [
      { id: "overview", label: "Overview", hint: "service inventory + QPS" },
      { id: "service-detail", label: "Service drill-down", hint: "selected service" },
      { id: "explore", label: "Explore", hint: "logs / metrics / traces" },
      { id: "logs", label: "Logs", hint: "search + live tail" },
      { id: "metrics", label: "Metrics", hint: "charts + histograms" },
      { id: "traces", label: "Traces", hint: "trace list + detail" },
      { id: "service-map", label: "Service map", hint: "caller → callee graph" },
      { id: "live", label: "Live tail", hint: "raw event stream" },
      { id: "dashboards", label: "Dashboards", hint: "built-in panels" },
      { id: "ingest", label: "Ingest demo", hint: "OTLP payloads + seed" },
      { id: "datasources", label: "Data sources & API", hint: "endpoints + /metrics" },
    ];
    const out: PaletteItem[] = basePages.map((p) => ({
      id: `page:${p.id}`,
      label: p.label,
      hint: p.hint,
      section: "Pages",
      perform: () => navigate(p.id),
    }));
    for (const s of services) {
      out.push({
        id: `svc:${s.name}`,
        label: `Service · ${s.name}`,
        hint: `p99 ${s.p99_ms.toFixed(0)}ms · ${(s.error_rate * 100).toFixed(1)}% err`,
        section: "Services",
        perform: () => {
          onServiceChange(s.name);
          navigate("service-detail");
        },
      });
    }
    // API shortcuts
    out.push(
      {
        id: "api:health",
        label: "GET /api/health",
        hint: "engine status",
        section: "API",
        perform: () => window.open("/api/health", "_blank"),
      },
      {
        id: "api:metrics",
        label: "GET /metrics",
        hint: "Prometheus exposition",
        section: "API",
        perform: () => window.open("/metrics", "_blank"),
      },
      {
        id: "api:labels",
        label: "GET /api/labels",
        hint: "known attribute keys",
        section: "API",
        perform: () => window.open("/api/labels", "_blank"),
      },
      {
        id: "api:servicemap",
        label: "GET /api/service-map",
        hint: "caller/callee edges",
        section: "API",
        perform: () => window.open("/api/service-map", "_blank"),
      },
      {
        id: "api:severity",
        label: "GET /api/severity",
        hint: "per-level counts",
        section: "API",
        perform: () => window.open("/api/severity", "_blank"),
      },
      {
        id: "api:snapshot",
        label: "GET /api/snapshot",
        hint: "live-tail backfill",
        section: "API",
        perform: () => window.open("/api/snapshot", "_blank"),
      },
      {
        id: "api:stream",
        label: "GET /api/stream",
        hint: "WebSocket event stream",
        section: "API",
        perform: () => window.open("/api/stream", "_blank"),
      }
    );
    return out;
  }, [services, navigate, onServiceChange]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return items;
    return items.filter(
      (it) =>
        it.label.toLowerCase().includes(q) ||
        it.hint?.toLowerCase().includes(q) ||
        it.section.toLowerCase().includes(q)
    );
  }, [items, query]);

  // Group items by section for display.
  const grouped = useMemo(() => {
    const out: Array<{ section: string; items: PaletteItem[] }> = [];
    for (const it of filtered) {
      const last = out[out.length - 1];
      if (last && last.section === it.section) last.items.push(it);
      else out.push({ section: it.section, items: [it] });
    }
    return out;
  }, [filtered]);

  // Keyboard navigation inside the palette.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setCursor((c) => Math.min(c + 1, filtered.length - 1));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setCursor((c) => Math.max(c - 1, 0));
      } else if (e.key === "Enter") {
        e.preventDefault();
        const item = filtered[cursor];
        if (item) {
          item.perform();
          setOpen(false);
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, filtered, cursor]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-[12vh] bg-black/40 backdrop-blur-sm"
      onClick={() => setOpen(false)}
    >
      <div
        className="bg-grafana-panel border border-grafana-border rounded-lg shadow-2xl w-[640px] max-w-[92vw] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 px-4 py-3 border-b border-grafana-border">
          <span className="text-grafana-muted font-mono text-[11px]">⌘K</span>
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setCursor(0);
            }}
            placeholder="Jump to page, service, or API…"
            className="flex-1 bg-transparent outline-none text-grafana-text placeholder:text-grafana-muted text-sm"
          />
          <button
            onClick={() => setOpen(false)}
            className="text-[11px] text-grafana-muted hover:text-grafana-text"
          >
            esc
          </button>
        </div>
        <div className="max-h-[55vh] overflow-y-auto scrollbar-thin py-1">
          {grouped.length === 0 ? (
            <div className="px-4 py-6 text-center text-grafana-muted italic text-sm">
              No matches for “{query}”.
            </div>
          ) : (
            grouped.map((g) => (
              <div key={g.section}>
                <div className="px-4 py-1 text-[10px] uppercase tracking-wider text-grafana-muted">
                  {g.section}
                </div>
                {g.items.map((it) => {
                  const idx = filtered.indexOf(it);
                  const active = idx === cursor;
                  return (
                    <button
                      key={it.id}
                      onMouseEnter={() => setCursor(idx)}
                      onClick={() => {
                        it.perform();
                        setOpen(false);
                      }}
                      className={`w-full text-left px-4 py-1.5 flex items-center justify-between text-sm ${
                        active
                          ? "bg-grafana-accent/15 text-grafana-accent"
                          : "text-grafana-text hover:bg-grafana-elev/50"
                      }`}
                    >
                      <span className="truncate">{it.label}</span>
                      {it.hint && (
                        <span className="text-[11px] text-grafana-muted ml-2 truncate max-w-[40%]">
                          {it.hint}
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            ))
          )}
        </div>
        <div className="border-t border-grafana-border px-4 py-2 text-[10px] text-grafana-muted flex items-center gap-3">
          <span>
            <kbd className="bg-grafana-elev px-1 py-0.5 rounded mr-1">↑↓</kbd> navigate
          </span>
          <span>
            <kbd className="bg-grafana-elev px-1 py-0.5 rounded mr-1">↵</kbd> open
          </span>
          <span>
            <kbd className="bg-grafana-elev px-1 py-0.5 rounded mr-1">esc</kbd> close
          </span>
          <span className="ml-auto">{filtered.length} matches</span>
        </div>
      </div>
    </div>
  );
}
