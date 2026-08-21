import type {
  DataSource,
  Dashboard,
  HealthResponse,
  HistogramResponse,
  LabelKeysResponse,
  Panel,
  QueryResult,
  SeverityResponse,
  ServiceMap,
  ServiceSummary,
  QPSResponse,
  SnapshotResponse,
  ServiceDetail,
  SpanRecord,
  AlertRule,
  AlertFire,
  Tenant,
  AuditEntry,
  AuditStats,
  SLOBudget,
  SLODecision,
  RetentionPolicy,
  QuotaStatus,
  CircuitSnapshot,
  ReplicaStatus,
  WebhookSubscriber,
  WebhookDelivery,
  WebhookStats,
  ProbeResult,
  OIDCDiscovery,
  OIDCProviderConfig,
} from "@/types/api";
import { apiFetch } from "./fetch";

const API_BASE = "/api";

export interface QueryParams {
  type: "logs" | "metrics" | "traces";
  service?: string;
  name?: string;
  severity?: string;
  trace_id?: string;
  search?: string;
  window?: "1m" | "5m";
  since?: number;
  until?: number;
  labels?: Record<string, string>;
  limit?: number;
  tenant?: string;
}

export function buildQuery(p: QueryParams): URLSearchParams {
  const out = new URLSearchParams();
  out.set("type", p.type);
  if (p.service) out.set("service", p.service);
  if (p.name) out.set("name", p.name);
  if (p.severity) out.set("severity", p.severity);
  if (p.trace_id) out.set("trace_id", p.trace_id);
  if (p.search) out.set("search", p.search);
  if (p.window) out.set("window", p.window);
  if (p.since) out.set("since", String(p.since));
  if (p.until) out.set("until", String(p.until));
  if (p.tenant) out.set("tenant", p.tenant);
  if (p.labels) {
    for (const [k, v] of Object.entries(p.labels)) {
      out.append("label", `${k}=${v}`);
    }
  }
  if (p.limit) out.set("limit", String(p.limit));
  return out;
}

function qs(p: QueryParams): string {
  const s = buildQuery(p).toString();
  return s ? `?${s}` : "";
}

export const apiService = {
  health: () => apiFetch<HealthResponse>("/health", { anonymous: true }),
  services: (tenant?: string) => {
    const q = tenant ? `?tenant=${encodeURIComponent(tenant)}` : "";
    return apiFetch<{ services: ServiceSummary[]; count: number }>(
      `/services${q}`
    );
  },
  service: (name: string, tenant?: string) => {
    const q = tenant ? `?tenant=${encodeURIComponent(tenant)}` : "";
    return apiFetch<ServiceSummary>(`/services/${name}${q}`);
  },
  query: (params: QueryParams) =>
    apiFetch<QueryResult>(`/query${qs(params)}`),
  datasources: () =>
    apiFetch<{ datasources: DataSource[]; count: number }>("/datasources"),
  dashboards: () => apiFetch<{ dashboards: Dashboard[] }>("/dashboards"),
  panels: (id: string) =>
    apiFetch<{ dashboard_id: string; panels: Panel[] }>(
      `/dashboards/${id}/panels`
    ),
  ingest: (body: unknown) =>
    apiFetch<unknown>("/ingest/otlp", { method: "POST", body }),
  ingestOTLPJSON: (body: unknown) =>
    apiFetch<unknown>("/ingest/otlp-json", { method: "POST", body }),
  seed: (service: string, n: number) =>
    apiFetch<{ service: string; seeded: number }>(
      `/seed?service=${encodeURIComponent(service)}&n=${n}`
    ),
  recentPayloads: () => apiFetch<{ payloads: unknown[] }>("/ingest/recent"),
  labels: () => apiFetch<LabelKeysResponse>("/labels"),
  serviceMap: () => apiFetch<ServiceMap>("/service-map"),
  trace: (id: string) =>
    apiFetch<{ trace_id: string; spans: SpanRecord[] }>(
      `/traces/${encodeURIComponent(id)}`
    ),
  qps: (windowMin = 5) =>
    apiFetch<QPSResponse>(`/qps?window_min=${windowMin}`),
  histogram: (service: string, bins = 20) =>
    apiFetch<HistogramResponse>(
      `/histogram?service=${encodeURIComponent(service)}&bins=${bins}`
    ),
  histogramOTel: (service: string, name: string) =>
    apiFetch<{
      service: string;
      name: string;
      bounds: number[];
      counts: number[];
      total: number;
      sum: number;
      min: number;
      max: number;
      p50: number;
      p95: number;
      p99: number;
    }>(
      `/histogram/otel?service=${encodeURIComponent(service)}&name=${encodeURIComponent(name)}`
    ),
  severity: (service = "") =>
    apiFetch<SeverityResponse>(
      service ? `/severity?service=${encodeURIComponent(service)}` : "/severity"
    ),
  snapshot: () => apiFetch<SnapshotResponse>("/snapshot"),
  metricNames: (limit = 50) =>
    apiFetch<{ names: string[] }>(`/metric-names?limit=${limit}`),
  exportUrl: (params: QueryParams & { format: "csv" | "json" }) =>
    `${API_BASE}/export?${buildQuery(params).toString()}&format=${params.format}`,
  serviceDetail: (name: string) => {
    if (!name) {
      return Promise.reject(new Error("serviceDetail: empty name"));
    }
    return apiFetch<ServiceDetail>(
      `/services/${encodeURIComponent(name)}/detail`
    );
  },
  alertsRules: () =>
    apiFetch<{ rules: AlertRule[] }>("/alerts/rules"),
  alertsRule: (name: string) =>
    apiFetch<AlertRule>(`/v1/rules/${encodeURIComponent(name)}`),
  upsertAlertRule: (rule: AlertRule) =>
    apiFetch<AlertRule>("/v1/rules", { method: "PUT", body: rule }),
  deleteAlertRule: (name: string) =>
    apiFetch<void>(`/v1/rules/${encodeURIComponent(name)}`, {
      method: "DELETE",
    }),
  alertsFires: (n = 100) =>
    apiFetch<{ fires: AlertFire[] }>(`/alerts/fires?n=${n}`),
  tenantsList: () =>
    apiFetch<{ tenants: Tenant[] }>("/tenants"),
  createTenant: (body: { id: string; name: string; description?: string }) =>
    apiFetch<Tenant>("/tenants", { method: "POST", body }),
  updateTenant: (id: string, body: Partial<Tenant>) =>
    apiFetch<Tenant>(`/tenants/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body,
    }),
  deleteTenant: (id: string) =>
    apiFetch<void>(`/tenants/${encodeURIComponent(id)}`, { method: "DELETE" }),
  listTenantKeys: (tenantId: string) =>
    apiFetch<{ keys: Array<{ id: string; label: string; role: string }> }>(
      `/tenants/${encodeURIComponent(tenantId)}/keys`
    ),
  mintTenantKey: (tenantId: string, body: { label: string; role: string }) =>
    apiFetch<{
      tenant_id: string;
      label: string;
      plaintext: string;
      role: string;
      created_at: string;
    }>(`/tenants/${encodeURIComponent(tenantId)}/keys`, { method: "POST", body }),
  rotateTenantKey: (tenantId: string, keyId: string) =>
    apiFetch<{ key_id: string; plaintext: string }>(
      `/tenants/${encodeURIComponent(tenantId)}/keys/${encodeURIComponent(keyId)}/rotate`,
      { method: "POST" }
    ),
  revokeTenantKey: (tenantId: string, keyId: string) =>
    apiFetch<void>(
      `/tenants/${encodeURIComponent(tenantId)}/keys/${encodeURIComponent(keyId)}`,
      { method: "DELETE" }
    ),

  // Round 39 / 41 / 42 surfaces
  audit: (limit = 200) =>
    apiFetch<{ entries: AuditEntry[] }>(`/audit?limit=${limit}`),
  auditStats: () => apiFetch<AuditStats>("/audit/stats"),
  probe: (target: string) =>
    apiFetch<ProbeResult>(
      `/probe?target=${encodeURIComponent(target)}`
    ),
  quota: (tenant: string) =>
    apiFetch<QuotaStatus>(
      `/v1/quota?tenant=${encodeURIComponent(tenant)}`
    ),
  quotaAll: () => apiFetch<{ quotas: QuotaStatus[] }>("/v1/quota"),

  // Round 44 SLO
  slos: () => apiFetch<{ slos: SLOBudget[] }>("/v1/slos"),
  sloDecide: (shortNs: number, longNs: number) =>
    apiFetch<SLODecision>(
      `/v1/slos/decide?short_ns=${shortNs}&long_ns=${longNs}`
    ),

  // Round 46 admin API keys
  adminKeys: () => apiFetch<{ keys: Array<{ id: string; label: string; tenant: string; role: string; created_at: string }> }>("/admin/keys"),
  createAdminKey: (body: { label: string; tenant: string; role: string; scopes?: string[]; ttl_ns?: number }) =>
    apiFetch<{ id: string; plaintext: string }>("/admin/keys", { method: "POST", body }),
  rotateAdminKey: (id: string, grace_ns = 0) =>
    apiFetch<{ id: string; plaintext: string }>(`/admin/keys/${encodeURIComponent(id)}/rotate?grace_ns=${grace_ns}`, { method: "POST" }),
  revokeAdminKey: (id: string) =>
    apiFetch<void>(`/admin/keys/${encodeURIComponent(id)}`, { method: "DELETE" }),

  // Round 47 circuit breaker
  circuits: () => apiFetch<{ circuits: Record<string, CircuitSnapshot> }>("/v1/circuits"),
  resetCircuit: (name: string) =>
    apiFetch<void>(`/v1/circuits/${encodeURIComponent(name)}/reset`, { method: "POST" }),

  // Round 48 ratelimit
  ratelimits: () => apiFetch<{ buckets: Array<{ key: string; tokens: number; level: number }> }>("/v1/ratelimits"),

  // Round 49 webhooks
  webhookSubscribers: () =>
    apiFetch<{ subscribers: WebhookSubscriber[] }>("/v1/webhooks"),
  addWebhookSubscriber: (s: WebhookSubscriber) =>
    apiFetch<WebhookSubscriber>("/v1/webhooks", { method: "POST", body: s }),
  removeWebhookSubscriber: (id: string) =>
    apiFetch<void>(`/v1/webhooks/${encodeURIComponent(id)}`, { method: "DELETE" }),
  webhookDLQ: () => apiFetch<{ deliveries: WebhookDelivery[] }>("/v1/webhooks/dlq"),
  webhookStats: () => apiFetch<WebhookStats>("/v1/webhooks/stats"),
  testWebhook: (id: string, body: { type: string; payload: Record<string, string>; tenant?: string }) =>
    apiFetch<WebhookDelivery>(`/v1/webhooks/${encodeURIComponent(id)}/test`, { method: "POST", body }),

  // Round 50 retention
  retentionList: () => apiFetch<{ policies: RetentionPolicy[] }>("/v1/retention"),
  setRetention: (body: RetentionPolicy) =>
    apiFetch<RetentionPolicy>("/v1/retention", { method: "PUT", body }),
  retentionReport: (tenant: string) =>
    apiFetch<{
      tenant: string;
      tier: string;
      hot_ns: number;
      cold_ns: number;
      drop: number;
      move: number;
      keep: number;
    }>(`/v1/retention/${encodeURIComponent(tenant)}/report`),

  // Round 43 backup
  backups: () => apiFetch<{ backups: Array<{ name: string; path: string; size: number; mod_time: string }> }>("/v1/backups"),
  createBackup: (body: { output: string; compress: boolean }) =>
    apiFetch<{ output: string; sha256: string; bytes: number; snapshot_id: string; taken_at: string }>(
      "/v1/backups",
      { method: "POST", body }
    ),
  verifyBackup: (path: string) =>
    apiFetch<{ ok: boolean; error?: string }>(
      `/v1/backups/verify?path=${encodeURIComponent(path)}`
    ),
  restoreBackup: (body: { path: string; into: string; dry_run: boolean }) =>
    apiFetch<{
      input: string;
      snapshot_id: string;
      restored_files: string[];
      taken_at: string;
      sha256: string;
    }>("/v1/backups/restore", { method: "POST", body }),

  // Replica state
  replicaStatus: () => apiFetch<ReplicaStatus>("/replica/state"),

  // OIDC
  oidcDiscovery: (issuer: string) =>
    apiFetch<OIDCDiscovery>(
      `/v1/auth/oidc/discovery?issuer=${encodeURIComponent(issuer)}`
    ),
  oidcProviders: () =>
    apiFetch<{ providers: OIDCProviderConfig[] }>("/v1/auth/oidc"),
  upsertOIDCProvider: (p: OIDCProviderConfig) =>
    apiFetch<OIDCProviderConfig>("/v1/auth/oidc", { method: "PUT", body: p }),
  deleteOIDCProvider: (issuer: string) =>
    apiFetch<void>(
      `/v1/auth/oidc?issuer=${encodeURIComponent(issuer)}`,
      { method: "DELETE" }
    ),

  // Prometheus 兼容端点
  // /api/v1/series 与 /api/v1/metadata 用于 grafana 等外部工具发现指标
  // /api/v1/query 与 /api/v1/write 兼容 prometheus HTTP API
  promSeries: (match = "{}") =>
    apiFetch<{ status: string; data: Array<{ __name__: string; service?: string }> }>(
      `/v1/series?match[]=${encodeURIComponent(match)}`
    ),
  promMetadata: () =>
    apiFetch<{ status: string; data: Record<string, Array<{ type: string; help: string }>> }>(
      `/v1/metadata`
    ),
  promQuery: (query: string, time?: number) =>
    apiFetch<{ status: string; data: { resultType: string; result: unknown[] } }>(
      `/v1/query?query=${encodeURIComponent(query)}${time ? `&time=${time}` : ""}`
    ),
  promRemoteWrite: (body: string, isJson = false) =>
    apiFetch<unknown>(
      isJson ? `/v1/write` : `/prom/write`,
      { method: "POST", body, headers: { "Content-Type": "application/x-protobuf" } }
    ),

  // /api/keys 列出现有 API key(只读 + 隐藏前缀)
  listKeys: () =>
    apiFetch<{ count: number; keys: Array<{ label: string; role: string; key_prefix: string }> }>(
      `/keys`
    ),

  // /api/dashboards/{id}/panels 已经在 panels() 中暴露
};

// Legacy alias — pages that imported `api` keep working while new
// code migrates to `apiService`.
export const api = apiService;

export const promMetrics = () => fetch("/metrics").then((r) => r.text());
