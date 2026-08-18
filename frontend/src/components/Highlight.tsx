import type { CSSProperties, ReactNode } from "react";

interface Props {
  text: string;
  /** Case-insensitive substring or regex source. */
  query?: string;
  className?: string;
  style?: CSSProperties;
}

// Highlight renders text with the matched portion of `query` wrapped in a
// styled <mark>. The match is case-insensitive and substring-based; pass a
// plain string for simple highlighting. Falls back to <span> for empty query.
export default function Highlight({ text, query, className, style }: Props): ReactNode {
  if (!query) return <span className={className} style={style}>{text}</span>;
  const needle = query;
  const lowerText = text.toLowerCase();
  const lowerNeedle = needle.toLowerCase();
  if (lowerNeedle === "") return <span className={className} style={style}>{text}</span>;

  const parts: ReactNode[] = [];
  let i = 0;
  let lastIndex = 0;
  while (i <= text.length) {
    const idx = lowerText.indexOf(lowerNeedle, i);
    if (idx < 0) {
      parts.push(text.slice(lastIndex));
      break;
    }
    if (idx > lastIndex) parts.push(text.slice(lastIndex, idx));
    parts.push(
      <mark
        key={idx}
        className="bg-grafana-yellow/30 text-grafana-yellow rounded px-0.5"
      >
        {text.slice(idx, idx + needle.length)}
      </mark>
    );
    lastIndex = idx + needle.length;
    i = lastIndex;
  }
  return <span className={className} style={style}>{parts}</span>;
}
