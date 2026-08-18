import type { Row } from "@/types/api";

interface Column {
  key: string;
  label: string;
  width?: string;
  render?: (row: Row) => React.ReactNode;
  mono?: boolean;
}

interface Props {
  rows: Row[];
  columns: Column[];
  emptyHint?: string;
  maxHeight?: number;
}

export default function Table({ rows, columns, emptyHint, maxHeight }: Props) {
  return (
    <div
      className="bg-grafana-panel border border-grafana-border rounded-md overflow-hidden"
      style={maxHeight ? { maxHeight, overflowY: "auto" } : undefined}
    >
      <table className="w-full text-sm">
        <thead className="bg-grafana-elev sticky top-0 z-10">
          <tr className="text-left text-[11px] text-grafana-muted uppercase tracking-wider">
            {columns.map((c) => (
              <th
                key={c.key}
                className="px-3 py-2 font-medium"
                style={{ width: c.width }}
              >
                {c.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && (
            <tr>
              <td
                colSpan={columns.length}
                className="px-3 py-6 text-center text-grafana-muted italic"
              >
                {emptyHint ?? "no rows"}
              </td>
            </tr>
          )}
          {rows.map((row, idx) => (
            <tr
              key={idx}
              className="border-t border-grafana-border hover:bg-grafana-elev/50"
            >
              {columns.map((c) => (
                <td
                  key={c.key}
                  className={`px-3 py-1.5 align-top ${
                    c.mono ? "font-mono text-[12px]" : ""
                  }`}
                >
                  {c.render
                    ? c.render(row)
                    : row[c.key] === undefined || row[c.key] === null
                    ? "-"
                    : String(row[c.key])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
