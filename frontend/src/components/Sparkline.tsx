// Tiny dependency-free SVG sparkline that renders an array of numbers as a
// single polyline. Used by the Sidebar to show recent activity per service.

interface Props {
  points: number[];
  width?: number;
  height?: number;
  color?: string;
  fill?: boolean;
}

export default function Sparkline({
  points,
  width = 64,
  height = 18,
  color = "#3b82f6",
  fill = true,
}: Props) {
  if (points.length === 0) {
    return (
      <svg width={width} height={height} className="block">
        <rect width={width} height={height} fill="transparent" />
      </svg>
    );
  }
  const min = Math.min(...points);
  const max = Math.max(...points);
  const range = Math.max(1, max - min);
  const stepX = width / Math.max(1, points.length - 1);
  const path = points
    .map((v, i) => {
      const x = i * stepX;
      const y = height - ((v - min) / range) * (height - 2) - 1;
      return `${i === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
    })
    .join(" ");
  const areaPath = points.length > 1 ? `${path} L ${width} ${height} L 0 ${height} Z` : path;
  return (
    <svg width={width} height={height} className="block">
      {fill && <path d={areaPath} fill={color} opacity={0.18} />}
      <path d={path} stroke={color} strokeWidth={1.2} fill="none" />
    </svg>
  );
}
