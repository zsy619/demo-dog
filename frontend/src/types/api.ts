// Shared types between frontend and backend
export type Severity = "TRACE" | "DEBUG" | "INFO" | "WARN" | "ERROR" | "FATAL";

export interface LogRecord {
  timestamp: string;
  service: string;
  severity: Severity;
  body: string;
  trace_id?: string;
  span_id?: string;
  attributes?: Record<string, string>;
}

export interface MetricPoint {
  timestamp: string;
  service: string;
  name: string;
  value: number;
  unit?: string;
  type: string;
  labels?: Record<string, string>;
}

export interface SpanRecord {
  trace_id: string;
  span_id: string;
  parent_id?: string;
  name: string;
  service: string;
  start_time: string;
  duration_ms: number;
  status: string;
  attributes?: Record<string, string>;
}

export interface ServiceSummary {
  name: string;
  logs_count: number;
  metrics_count: number;
  spans_count: number;
  error_rate: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  qps: number;
  updated_at: string;
  last_labels?: string[];
}

export interface SeriesPoint {
  ts: number;
  value: number;
}

export interface MetricSeries {
  name: string;
  service: string;
  unit?: string;
  labels?: Record<string, string>;
  points: SeriesPoint[];
}

export type Row = Record<string, unknown>;

export interface QueryStats {
  scanned: number;
  returned: number;
  took_ms: number;
  tier: string;
  mv_used?: string;
}

export interface QueryResult {
  type: "logs" | "metrics" | "traces";
  rows?: Row[];
  series?: MetricSeries[];
  stats: QueryStats;
}

export interface DataSource {
  id: string;
  name: string;
  type: string;
  default?: boolean;
  url: string;
  database: string;
  tables: string[];
  capabilities: string[];
  description: string;
  version: string;
  plugin_version: string;
}

export interface Dashboard {
  id: string;
  name: string;
  description: string;
  tags: string[];
}

export interface Panel {
  id: string;
  type: string;
  title: string;
  datasource: string;
  query: string;
  grid?: { x: number; y: number; w: number; h: number };
  // Optional per-panel config the frontend can interpret.
  config?: {
    service?: string;
    metric?: string;
    window?: string;
    severity?: string;
    trace_id?: string;
    max_rows?: number;
  };
}

export interface HealthResponse {
  status: string;
  uptime: string;
  engine: {
    logs_accepted: number;
    metrics_accepted: number;
    spans_accepted: number;
    queries_served: number;
    hot_logs: number;
    cold_logs: number;
    hot_metrics: number;
    cold_metrics: number;
    hot_spans: number;
    cold_spans: number;
    services: number;
  };
  version: string;
  now: string;
}

export interface StreamEvent {
  kind: "log" | "metric" | "span" | "service" | "hello";
  service: string;
  timestamp: number;
  body?: string;
  value?: number;
  name?: string;
  status?: string;
  trace_id?: string;
  span_id?: string;
}

export interface LabelKeysResponse {
  logs: string[];
  metrics: string[];
  spans: string[];
}

export interface ServiceMapEdge {
  from: string;
  to: string;
  calls: number;
  errors: number;
  avg_ms: number;
  p99_ms: number;
}

export interface ServiceMap {
  edges: ServiceMapEdge[];
  nodes: string[];
}

export interface QPSResponse {
  window_min: number;
  series: Array<{ service: string; points: SeriesPoint[] }>;
}

export interface HistogramResponse {
  service: string;
  bins: number;
  counts: number[];
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
}

export interface SeverityResponse {
  service: string;
  counts: Record<string, number>;
}

export interface SnapshotResponse {
  logs: LogRecord[];
  metrics: MetricPoint[];
  spans: SpanRecord[];
}

// Per-service drill-down payload from /api/services/{name}/detail.
export interface EndpointStats {
  name: string;
  count: number;
  errors: number;
  avg_ms: number;
  p99_ms: number;
}

export interface ServiceDetail {
  summary: ServiceSummary;
  top_ops: EndpointStats[];
  metric_names: string[];
  recent_errors: LogRecord[];
  recent_traces: string[];
  endpoints: EndpointStats[];
  qps: SeriesPoint[];
}

export interface AlertRule {
  name: string;
  description?: string;
  service?: string;
  target: number;
  // durations come over the wire as nanoseconds (Go time.Duration).
  window: number;
  fast_window: number;
  fast_burn: number;
  slow_burn: number;
  severity: "info" | "warning" | "critical";
  channels: string[];
}

export interface AlertFire {
  rule: AlertRule;
  severity: string;
  timestamp: string;
  window: "fast" | "slow";
  burn_rate: number;
  reason: string;
}

export interface AlertRule {
  name: string;
  description?: string;
  service?: string;
  target: number;
  // Durations arrive as nanoseconds (Go time.Duration).
  window: number;
  fast_window: number;
  fast_burn: number;
  slow_burn: number;
  severity: "info" | "warning" | "critical";
  channels: string[];
}

export interface AlertFire {
  rule: AlertRule;
  severity: string;
  timestamp: string;
  window: "fast" | "slow";
  burn_rate: number;
  reason: string;
}

export interface AlertRule {
  name: string;
  description?: string;
  service?: string;
  target: number;
  window: number;
  fast_window: number;
  fast_burn: number;
  slow_burn: number;
  severity: "info" | "warning" | "critical";
  channels: string[];
}

export interface AlertFire {
  rule: AlertRule;
  severity: string;
  timestamp: string;
  window: "fast" | "slow";
  burn_rate: number;
  reason: string;
}

export interface Tenant {
  id: string;
  name: string;
  description?: string;
  created_at: string;
  active: boolean;
}
