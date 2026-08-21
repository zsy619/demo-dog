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
  // Durations arrive as nanoseconds (Go time.Duration).
  window: number;
  fast_window: number;
  fast_burn: number;
  slow_burn: number;
  severity: "info" | "warning" | "critical";
  // 后端 alerts.Rule 暂无 channels 字段;保留为可选以保持向前兼容。
  channels?: string[];
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

// Admin / SLO / webhook / retention / quota / circuit / replica
// surfaces that align with backend Rounds 42-50.

export interface AuditEntry {
  ts: string;
  actor: string;
  action: string;
  target?: string;
  ip?: string;
  ok: boolean;
  error?: string;
}

export interface AuditStats {
  total: number;
  ok: number;
  failed: number;
  by_action: Record<string, number>;
  // 后端当前未返回 by_actor;保留为可选以保持向后兼容。
  by_actor?: Record<string, number>;
}

export interface SLOBudget {
  name: string;
  service: string;
  target: number;
  total: number;
  bad: number;
  error_rate: number;
  budget: number;
  budget_left: number;
  budget_left_percent: number;
  healthy: boolean;
  as_of: string;
  score: number;
}

export interface SLODecision {
  short_window_ns: number;
  short_burn: number;
  long_window_ns: number;
  long_burn: number;
  level: "none" | "warn" | "page";
  reason: string;
}

export interface RetentionPolicy {
  tenant: string;
  tier: "free" | "pro" | "enterprise" | string;
  hot_ttl_ns: number;
  cold_ttl_ns: number;
  updated_at: string;
}

export interface BillingUsage {
  tenant: string;
  metric: string;
  periods: Record<string, number>; // period (YYYY-MM) → value
  total: number;
}

export interface BillingPeriodTotal {
  tenant: string;
  metric: string;
  period: string; // YYYY-MM
  value: number;
  updated_at: string;
}

export interface BillingPointResponse {
  tenant: string;
  metric: string;
  period: string;
  value: number;
  present: boolean;
}

export interface BillingTenantResponse {
  tenant: string;
  usage: BillingUsage[];
}

export interface BillingMetricsResponse {
  tenant: string;
  period: string;
  metrics: BillingPeriodTotal[];
}

export interface BillingAllResponse {
  rows: BillingPeriodTotal[];
}

export interface BillingRecordResponse {
  tenant: string;
  metric: string;
  delta: number;
}

export interface QuotaStatus {
  tenant: string;
  requests: number;
  bytes: number;
  limited: number;
  max_requests: number;
  max_bytes: number;
}

export interface CircuitSnapshot {
  state: "closed" | "open" | "half_open";
  failures: number;
  threshold: number;
  cool_down_ns: number;
  opened_at?: string;
}

export interface ReplicaStatus {
  role: "primary" | "follower" | "";
  peers: Array<{ id: string; last_offset: number; last_ack: number }>;
  pending: number;
  committed: number;
}

export interface WebhookSubscriber {
  id: string;
  url: string;
  secret: string;
  event_types: string[];
  max_retries: number;
}

export interface WebhookDelivery {
  event_id: string;
  subscriber_id: string;
  attempts: number;
  status: number;
  error?: string;
  latency_ns: number;
  last_try: string;
}

export interface WebhookStats {
  subscribers: number;
  delivered: number;
  failed: number;
  dlq: number;
}

export interface ProbeResult {
  ok: boolean;
  duration_ns: number;
  status_code: number;
  target: string;
}

export interface OIDCDiscovery {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  jwks_uri: string;
  userinfo_endpoint: string;
}

export interface OIDCProviderConfig {
  issuer: string;
  client_id: string;
  audiences: string[];
  scopes: string[];
  enabled: boolean;
  email_claim: string;
  groups_claim: string;
}
