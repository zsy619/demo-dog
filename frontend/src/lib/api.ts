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
  alertsFires: (n = 100) =>
    apiFetch<{ fires: AlertFire[] }>(`/alerts/fires?n=${n}`),
  tenantsList: () =>
    apiFetch<{ tenants: Tenant[] }>("/tenants"),
  createTenant: (body: { id: string; name: string; description?: string }) =>
    apiFetch<Tenant>("/tenants", { method: "POST", body }),
  mintTenantKey: (tenantId: string, body: { label: string; role: string }) =>
    apiFetch<{
      tenant_id: string;
      label: string;
      plaintext: string;
      role: string;
      created_at: string;
    }>(`/tenants/${encodeURIComponent(tenantId)}/keys`, { method: "POST", body }),
};

// Legacy alias — pages that imported `api` keep working while new
// code migrates to `apiService`.
export const api = apiService;

export const promMetrics = () => fetch("/metrics").then((r) => r.text());
