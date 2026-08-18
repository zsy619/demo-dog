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
