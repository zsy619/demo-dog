interface SkeletonProps {
  rows?: number;
  cols?: number;
  className?: string;
}

// Skeleton renders a small loading placeholder grid.
export function Skeleton({ rows = 3, cols = 4, className = "" }: SkeletonProps) {
  return (
    <div className={`space-y-2 animate-pulse ${className}`}>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex gap-2">
          {Array.from({ length: cols }).map((_, j) => (
            <div
              key={j}
              className="h-4 flex-1 bg-grafana-elev rounded"
              style={{ minWidth: 40 }}
            />
          ))}
        </div>
      ))}
    </div>
  );
}

interface EmptyProps {
  title?: string;
  hint?: string;
  action?: { label: string; onClick: () => void };
}

// Empty renders a friendly "nothing here yet" placeholder.
export function Empty({ title = "No data", hint, action }: EmptyProps) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-12 px-4 text-grafana-text-secondary">
      <div className="text-3xl mb-2">📭</div>
      <div className="text-sm font-medium">{title}</div>
      {hint && <div className="text-xs mt-1 max-w-xs">{hint}</div>}
      {action && (
        <button
          onClick={action.onClick}
          className="mt-3 px-3 py-1 text-xs bg-grafana-blue text-white rounded hover:bg-grafana-blue/80"
        >
          {action.label}
        </button>
      )}
    </div>
  );
}

interface ErrorBoxProps {
  error: Error;
  onRetry?: () => void;
  className?: string;
}

// ErrorBox renders an inline error pill.
export function ErrorBox({ error, onRetry, className = "" }: ErrorBoxProps) {
  return (
    <div className={`flex items-center gap-2 px-3 py-2 my-2 bg-grafana-err/10 border border-grafana-err/30 rounded text-grafana-err ${className}`}>
      <span className="text-sm">⚠ {error.message}</span>
      {onRetry && (
        <button
          onClick={onRetry}
          className="ml-auto px-2 py-0.5 text-xs bg-grafana-elev border border-grafana-border rounded hover:bg-grafana-elev2"
        >
          Retry
        </button>
      )}
    </div>
  );
}
