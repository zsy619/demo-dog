// Typed wrappers for the most common API endpoints.
//
// Centralising the query keys here is what makes cache invalidation
// usable: any caller can do queryClient.invalidateQueries({ queryKey:
// queryKeys.services(tenant) }) and every screen that depends on
// the same data refetches once.

import { useApiQuery } from "./useApiQuery";
import type {
  DataSource,
  Dashboard,
  HealthResponse,
  LabelKeysResponse,
  Panel,
  QueryResult,
  ServiceMap,
  ServiceSummary,
  SpanRecord,
  QPSResponse,
  SeverityResponse,
  SnapshotResponse,
  AuditEntry,
  AuditStats,
  SLOBudget,
  RetentionPolicy,
  QuotaStatus,
  CircuitSnapshot,
  ReplicaStatus,
  WebhookSubscriber,
  WebhookDelivery,
  WebhookStats,
  OIDCProviderConfig,
} from "@/types/api";
import type { QueryParams } from "@/lib/api";

export const queryKeys = {
  health: () => ["health"] as const,
  services: (tenant?: string) => ["services", tenant ?? ""] as const,
  service: (name: string, tenant?: string) =>
    ["service", tenant ?? "", name] as const,
  serviceDetail: (name: string) => ["service-detail", name] as const,
  datasources: () => ["datasources"] as const,
  dashboards: () => ["dashboards"] as const,
  panels: (id: string) => ["dashboards", id, "panels"] as const,
  labels: () => ["labels"] as const,
  serviceMap: () => ["service-map"] as const,
  trace: (id: string) => ["trace", id] as const,
  qps: (windowMin: number) => ["qps", windowMin] as const,
  histogram: (service: string, bins: number) =>
    ["histogram", service, bins] as const,
  severity: (service: string) => ["severity", service] as const,
  snapshot: () => ["snapshot"] as const,
  metricNames: (limit: number) => ["metric-names", limit] as const,
  query: (params: QueryParams) => ["query", params] as const,
  // Round 39 / 41 / 42
  audit: (limit: number) => ["audit", limit] as const,
  auditStats: () => ["audit", "stats"] as const,
  quotaAll: () => ["quota", "all"] as const,
  quota: (tenant: string) => ["quota", tenant] as const,
  // Round 44
  slos: () => ["slos"] as const,
  // Round 46
  adminKeys: () => ["admin", "keys"] as const,
  // Round 47
  circuits: () => ["circuits"] as const,
  // Round 48
  ratelimits: () => ["ratelimits"] as const,
  // Round 49
  webhooks: () => ["webhooks"] as const,
  webhooksDLQ: () => ["webhooks", "dlq"] as const,
  webhooksStats: () => ["webhooks", "stats"] as const,
  // Round 50
  retention: () => ["retention"] as const,
  retentionReport: (tenant: string) => ["retention", tenant, "report"] as const,
  // Round 43
  backups: () => ["backups"] as const,
  // Replica
  replica: () => ["replica"] as const,
  // OIDC
  oidcProviders: () => ["oidc", "providers"] as const,
};

// ---- query hooks -----------------------------------------------------

export function useHealth() {
  return useApiQuery<HealthResponse>(queryKeys.health(), "/health", {
    refetchInterval: 5_000,
  });
}

export function useServices(tenant?: string) {
  return useApiQuery<{ services: ServiceSummary[]; count: number }>(
    queryKeys.services(tenant),
    `/services${tenant ? `?tenant=${encodeURIComponent(tenant)}` : ""}`
  );
}

export function useService(name: string, tenant?: string) {
  return useApiQuery<ServiceSummary>(
    queryKeys.service(name, tenant),
    `/services/${encodeURIComponent(name)}${tenant ? `?tenant=${encodeURIComponent(tenant)}` : ""}`
  );
}

export function useServiceDetail(name: string) {
  return useApiQuery(
    queryKeys.serviceDetail(name),
    `/services/${encodeURIComponent(name)}/detail`,
    { skip: !name }
  );
}

export function useDatasources() {
  return useApiQuery<{ datasources: DataSource[]; count: number }>(
    queryKeys.datasources(),
    "/datasources"
  );
}

export function useDashboards() {
  return useApiQuery<{ dashboards: Dashboard[] }>(
    queryKeys.dashboards(),
    "/dashboards"
  );
}

export function usePanels(id: string) {
  return useApiQuery<{ dashboard_id: string; panels: Panel[] }>(
    queryKeys.panels(id),
    `/dashboards/${id}/panels`
  );
}

export function useLabels() {
  return useApiQuery<LabelKeysResponse>(queryKeys.labels(), "/labels");
}

export function useServiceMap() {
  return useApiQuery<ServiceMap>(queryKeys.serviceMap(), "/service-map");
}

export function useTrace(id: string) {
  return useApiQuery<{ trace_id: string; spans: SpanRecord[] }>(
    queryKeys.trace(id),
    `/traces/${encodeURIComponent(id)}`,
    { skip: !id }
  );
}

export function useQps(windowMin = 5) {
  return useApiQuery<QPSResponse>(
    queryKeys.qps(windowMin),
    `/qps?window_min=${windowMin}`,
    { refetchInterval: 8_000 }
  );
}

export function useHistogram(service: string, bins = 20) {
  return useApiQuery(
    queryKeys.histogram(service, bins),
    `/histogram?service=${encodeURIComponent(service)}&bins=${bins}`,
    { skip: !service }
  );
}

export function useSeverity(service = "") {
  return useApiQuery<SeverityResponse>(
    queryKeys.severity(service),
    service ? `/severity?service=${encodeURIComponent(service)}` : "/severity"
  );
}

export function useSnapshot() {
  return useApiQuery<SnapshotResponse>(
    queryKeys.snapshot(),
    "/snapshot",
    { refetchInterval: 2_000 }
  );
}

export function useMetricNames(limit = 50) {
  return useApiQuery<{ names: string[] }>(
    queryKeys.metricNames(limit),
    `/metric-names?limit=${limit}`
  );
}

export function useQuery_(params: QueryParams) {
  return useApiQuery<QueryResult>(
    queryKeys.query(params),
    `/query?${new URLSearchParams(
      Object.entries({
        type: params.type,
        ...(params.service && { service: params.service }),
        ...(params.name && { name: params.name }),
        ...(params.severity && { severity: params.severity }),
        ...(params.trace_id && { trace_id: params.trace_id }),
        ...(params.search && { search: params.search }),
        ...(params.window && { window: params.window }),
        ...(params.since && { since: String(params.since) }),
        ...(params.until && { until: String(params.until) }),
        ...(params.tenant && { tenant: params.tenant }),
        ...(params.limit && { limit: String(params.limit) }),
      })
    ).toString()}`
  );
}

// ---- admin / ops hooks (Rounds 39-50) --------------------------------

export function useAudit(limit = 200) {
  return useApiQuery<{ entries: AuditEntry[] }>(
    queryKeys.audit(limit),
    `/audit?limit=${limit}`,
    { refetchInterval: 10_000 }
  );
}

export function useAuditStats() {
  return useApiQuery<AuditStats>(queryKeys.auditStats(), "/audit/stats", {
    refetchInterval: 10_000,
  });
}

export function useQuotaAll() {
  return useApiQuery<{ quotas: QuotaStatus[] }>(
    queryKeys.quotaAll(),
    "/v1/quota",
    { refetchInterval: 15_000 }
  );
}

export function useQuota(tenant: string) {
  return useApiQuery<QuotaStatus>(
    queryKeys.quota(tenant),
    `/v1/quota?tenant=${encodeURIComponent(tenant)}`,
    { skip: !tenant }
  );
}

export function useSLOs() {
  return useApiQuery<{ slos: SLOBudget[] }>(
    queryKeys.slos(),
    "/v1/slos",
    { refetchInterval: 30_000 }
  );
}

export function useAdminKeys() {
  return useApiQuery<{
    keys: Array<{ id: string; label: string; tenant: string; role: string; created_at: string }>;
  }>(queryKeys.adminKeys(), "/admin/keys");
}

export function useCircuits() {
  return useApiQuery<{ circuits: Record<string, CircuitSnapshot> }>(
    queryKeys.circuits(),
    "/v1/circuits",
    { refetchInterval: 15_000 }
  );
}

export function useRatelimits() {
  return useApiQuery<{ buckets: Array<{ key: string; tokens: number; level: number }> }>(
    queryKeys.ratelimits(),
    "/v1/ratelimits",
    { refetchInterval: 15_000 }
  );
}

export function useWebhooks() {
  return useApiQuery<{ subscribers: WebhookSubscriber[] }>(
    queryKeys.webhooks(),
    "/v1/webhooks"
  );
}

export function useWebhookDLQ() {
  return useApiQuery<{ deliveries: WebhookDelivery[] }>(
    queryKeys.webhooksDLQ(),
    "/v1/webhooks/dlq"
  );
}

export function useWebhookStats() {
  return useApiQuery<WebhookStats>(
    queryKeys.webhooksStats(),
    "/v1/webhooks/stats",
    { refetchInterval: 10_000 }
  );
}

export function useRetention() {
  return useApiQuery<{ policies: RetentionPolicy[] }>(
    queryKeys.retention(),
    "/v1/retention"
  );
}

export function useRetentionReport(tenant: string) {
  return useApiQuery<{
    tenant: string;
    tier: string;
    hot_ns: number;
    cold_ns: number;
    drop: number;
    move: number;
    keep: number;
  }>(queryKeys.retentionReport(tenant), `/v1/retention/${encodeURIComponent(tenant)}/report`, { skip: !tenant });
}

export function useBackups() {
  return useApiQuery<{
    backups: Array<{ name: string; path: string; size: number; mod_time: string }>;
  }>(queryKeys.backups(), "/v1/backups");
}

export function useReplicaStatus() {
  return useApiQuery<ReplicaStatus>(queryKeys.replica(), "/replica/state", {
    refetchInterval: 5_000,
  });
}

export function useOIDCProviders() {
  return useApiQuery<{ providers: OIDCProviderConfig[] }>(
    queryKeys.oidcProviders(),
    "/v1/auth/oidc"
  );
}
