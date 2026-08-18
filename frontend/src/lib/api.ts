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
} from "@/types/api";

const API_BASE = "/api";

export interface QueryParams {
  type: "logs" | "metrics" | "traces";
  service?: string;
  name?: string; // metric name
  severity?: string;
  trace_id?: string;
  search?: string;
  window?: "1m" | "5m";
  since?: number; // ms
  until?: number; // ms
  labels?: Record<string, string>;
  limit?: number;
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
  if (p.labels) {
    for (const [k, v] of Object.entries(p.labels)) {
      out.append("label", `${k}=${v}`);
    }
  }
  if (p.limit) out.set("limit", String(p.limit));
  return out;
}

async function getJson<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`GET ${path} failed: ${res.status} ${text}`);
  }
  return res.json() as Promise<T>;
}

async function postJson<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`POST ${path} failed: ${res.status} ${text}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  health: () => getJson<HealthResponse>("/health"),
  services: () =>
    getJson<{ services: ServiceSummary[]; count: number }>("/services"),
  service: (name: string) => getJson<ServiceSummary>(`/services/${name}`),
  query: (params: QueryParams) =>
    getJson<QueryResult>(`/query?${buildQuery(params).toString()}`),
  datasources: () =>
    getJson<{ datasources: DataSource[]; count: number }>("/datasources"),
  dashboards: () => getJson<{ dashboards: Dashboard[] }>("/dashboards"),
  panels: (id: string) =>
    getJson<{ dashboard_id: string; panels: Panel[] }>(
      `/dashboards/${id}/panels`
    ),
  ingest: (body: unknown) => postJson<unknown>("/ingest/otlp", body),
  ingestOTLPJSON: (body: unknown) =>
    postJson<unknown>(
      "/ingest/otlp-json",
      body
    ),
  seed: (service: string, n: number) =>
    getJson<{ service: string; seeded: number }>(
      `/seed?service=${encodeURIComponent(service)}&n=${n}`
    ),
  recentPayloads: () => getJson<{ payloads: unknown[] }>("/ingest/recent"),
  // New endpoints
  labels: () => getJson<LabelKeysResponse>("/labels"),
  serviceMap: () => getJson<ServiceMap>("/service-map"),
  trace: (id: string) =>
    getJson<{ trace_id: string; spans: import("@/types/api").SpanRecord[] }>(
      `/traces/${encodeURIComponent(id)}`
    ),
  qps: (windowMin = 5) => getJson<QPSResponse>(`/qps?window_min=${windowMin}`),
  histogram: (service: string, bins = 20) =>
    getJson<HistogramResponse>(
      `/histogram?service=${encodeURIComponent(service)}&bins=${bins}`
    ),
  severity: (service = "") =>
    getJson<SeverityResponse>(
      service
        ? `/severity?service=${encodeURIComponent(service)}`
        : "/severity"
    ),
  snapshot: () => getJson<SnapshotResponse>("/snapshot"),
  metricNames: (limit = 50) => getJson<{ names: string[] }>(`/metric-names?limit=${limit}`),
  exportUrl: (params: QueryParams & { format: "csv" | "json" }) =>
    `${API_BASE}/export?${buildQuery(params).toString()}&format=${params.format}`,
  serviceDetail: (name: string) => {
    // Refuse to fire a malformed URL when no service is picked. The page
    // also guards against this, but defending the api boundary keeps a
    // future caller from accidentally hitting the 404 endpoint.
    if (!name) {
      return Promise.reject(new Error("serviceDetail: empty name"));
    }
    return getJson<import("@/types/api").ServiceDetail>(
      `/services/${encodeURIComponent(name)}/detail`
    );
  },
};

export const promMetrics = () => fetch("/metrics").then((r) => r.text());
