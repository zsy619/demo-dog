// Generic virtualised table.
//
// Renders a fixed-height header row + a scrollable body whose
// visible rows are computed via @tanstack/react-virtual. We
// intentionally do NOT depend on a custom <table> wrapper
// because virtualising a real <table> requires resetting
// widths on every scroll, which we can do more cheaply with a
// div grid.
//
// Callers pass:
//   - rows:        the data array.
//   - columns:     a list of {key, header, width, render} tuples.
//   - rowHeight:   a fixed pixel height per row (default 28).
//   - threshold:   the row count above which we virtualise
//                  (default 1000). Below that we render flat.
//
// Anything below the threshold gets the cheap flat path so
// small tables do not pay the virtualisation overhead.

import { useRef, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

export interface VirtualColumn<T> {
  key: string;
  header: string;
  // Tailwind width class (e.g. "w-32") or a raw px string.
  widthClass?: string;
  widthPx?: number;
  render: (row: T, index: number) => React.ReactNode;
  align?: "left" | "right" | "center";
}

interface Props<T> {
  rows: T[];
  columns: VirtualColumn<T>[];
  rowHeight?: number;
  threshold?: number;
  className?: string;
  emptyMessage?: string;
  ariaLabel?: string;
}

export function VirtualTable<T>({
  rows,
  columns,
  rowHeight = 28,
  threshold = 1000,
  className = "",
  emptyMessage = "No data",
  ariaLabel,
}: Props<T>) {
  const useVirtual = rows.length >= threshold;
  const scrollRef = useRef<HTMLDivElement | null>(null);

  const virtualizer = useVirtualizer({
    count: useVirtual ? rows.length : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 8,
  });

  // Flat rendering: small tables get a normal table for screen
  // readers + simpler styling.
  if (!useVirtual) {
    return (
      <div className={`overflow-auto scrollbar-thin ${className}`}>
        <table className="w-full text-xs" aria-label={ariaLabel}>
          <thead>
            <tr className="text-left text-grafana-muted border-b border-grafana-border bg-grafana-elev/40">
              {columns.map((c) => (
                <th
                  key={c.key}
                  className={`px-3 py-2 ${c.widthClass ?? ""}`}
                  style={c.widthPx ? { width: c.widthPx } : undefined}
                >
                  {c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="px-3 py-6 text-center text-grafana-muted">
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              rows.map((row, i) => (
                <tr
                  key={i}
                  className="border-t border-grafana-border hover:bg-grafana-elev/50"
                >
                  {columns.map((c) => (
                    <td
                      key={c.key}
                      className={`px-3 py-1 ${c.widthClass ?? ""} ${
                        c.align === "right"
                          ? "text-right"
                          : c.align === "center"
                          ? "text-center"
                          : ""
                      }`}
                      style={c.widthPx ? { width: c.widthPx } : undefined}
                    >
                      {c.render(row, i)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    );
  }

  // Virtual rendering: header is a fixed div; body is a scroll
  // container with absolutely-positioned rows.
  const totalSize = virtualizer.getTotalSize();
  const items = virtualizer.getVirtualItems();
  return (
    <div className={`flex flex-col overflow-hidden ${className}`}>
      <div
        role="row"
        className="flex text-left text-grafana-muted text-xs border-b border-grafana-border bg-grafana-elev/40 shrink-0"
      >
        {columns.map((c) => (
          <div
            key={c.key}
            role="columnheader"
            className={`px-3 py-2 ${c.widthClass ?? ""} shrink-0`}
            style={c.widthPx ? { width: c.widthPx, flex: "0 0 auto" } : { flex: 1 }}
          >
            {c.header}
          </div>
        ))}
      </div>
      <div
        ref={scrollRef}
        className="flex-1 overflow-auto scrollbar-thin"
        role="grid"
        aria-label={ariaLabel}
        aria-rowcount={rows.length}
      >
        {rows.length === 0 ? (
          <div className="px-3 py-6 text-center text-grafana-muted text-xs">
            {emptyMessage}
          </div>
        ) : (
          <div style={{ height: totalSize, position: "relative" }}>
            {items.map((vi) => {
              const row = rows[vi.index];
              return (
                <div
                  key={vi.key}
                  role="row"
                  aria-rowindex={vi.index + 1}
                  className="flex text-xs border-t border-grafana-border hover:bg-grafana-elev/50 absolute top-0 left-0 w-full"
                  style={{
                    height: rowHeight,
                    transform: `translateY(${vi.start}px)`,
                  }}
                >
                  {columns.map((c) => (
                    <div
                      key={c.key}
                      role="gridcell"
                      className={`px-3 py-1 ${c.widthClass ?? ""} shrink-0 truncate ${
                        c.align === "right"
                          ? "text-right"
                          : c.align === "center"
                          ? "text-center"
                          : ""
                      }`}
                      style={
                        c.widthPx
                          ? { width: c.widthPx, flex: "0 0 auto" }
                          : { flex: 1 }
                      }
                      title={
                        typeof row === "object" && row !== null
                          ? undefined
                          : undefined
                      }
                    >
                      {c.render(row, vi.index)}
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// VirtualList — minimal row-only virtualiser for cases that are
// not table-shaped (e.g. the live event ticker).
export function VirtualList<T>({
  rows,
  rowHeight = 24,
  threshold = 200,
  renderRow,
  className = "",
  emptyMessage = "No data",
}: {
  rows: T[];
  rowHeight?: number;
  threshold?: number;
  renderRow: (row: T, index: number) => React.ReactNode;
  className?: string;
  emptyMessage?: string;
}) {
  const useVirtual = rows.length >= threshold;
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const virtualizer = useVirtualizer({
    count: useVirtual ? rows.length : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 6,
  });

  if (!useVirtual) {
    return (
      <div className={`overflow-auto ${className}`}>
        {rows.length === 0 ? (
          <div className="px-3 py-4 text-center text-grafana-muted text-xs">
            {emptyMessage}
          </div>
        ) : (
          rows.map((r, i) => (
            <div key={i} style={{ height: rowHeight }}>
              {renderRow(r, i)}
            </div>
          ))
        )}
      </div>
    );
  }

  return (
    <div
      ref={scrollRef}
      className={`overflow-auto scrollbar-thin ${className}`}
    >
      <div
        style={{ height: virtualizer.getTotalSize(), position: "relative" }}
      >
        {virtualizer.getVirtualItems().map((vi) => {
          const row = rows[vi.index];
          return (
            <div
              key={vi.key}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                height: rowHeight,
                transform: `translateY(${vi.start}px)`,
              }}
            >
              {renderRow(row, vi.index)}
            </div>
          );
        })}
      </div>
    </div>
  );
}
