import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: (err: Error, reset: () => void) => ReactNode;
}

interface State {
  err: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { err: null };

  static getDerivedStateFromError(err: Error): State {
    return { err };
  }

  componentDidCatch(err: Error, info: ErrorInfo) {
    // Surface the failure but don't crash the whole app.
    console.error("ErrorBoundary caught:", err, info);
  }

  reset = () => this.setState({ err: null });

  render() {
    if (this.state.err) {
      if (this.props.fallback) return this.props.fallback(this.state.err, this.reset);
      return (
        <div className="p-4 m-4 border border-grafana-err/40 bg-grafana-err/10 rounded text-grafana-err">
          <div className="font-medium mb-1">Something went wrong.</div>
          <div className="text-xs font-mono whitespace-pre-wrap">{this.state.err.message}</div>
          <button
            onClick={this.reset}
            className="mt-3 px-2 py-1 text-xs bg-grafana-elev border border-grafana-border rounded hover:bg-grafana-elev2"
          >
            Retry
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
