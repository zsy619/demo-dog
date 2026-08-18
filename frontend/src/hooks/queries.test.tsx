import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useHealth,
  useServices,
  useQps,
  useDatasources,
  useDashboards,
  useLabels,
  useHistogram,
  usePanels,
  queryKeys,
} from "./queries";
import { apiFetch } from "@/lib/fetch";

vi.mock("@/lib/fetch", () => ({
  apiFetch: vi.fn(),
  HttpError: class HttpError extends Error {
    status: number;
    constructor(status: number, msg: string) {
      super(msg);
      this.status = status;
    }
  },
}));

function makeWrapper() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return { Wrapper, qc };
}

describe("typed query hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("useHealth returns data and caches under queryKeys.health", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ status: "ok" });
    const { Wrapper, qc } = makeWrapper();
    const { result } = renderHook(() => useHealth(), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toEqual({ status: "ok" }));
    expect(apiFetch).toHaveBeenCalledWith("/health", expect.any(Object));
    expect(qc.getQueryData(queryKeys.health())).toEqual({ status: "ok" });
  });

  it("useServices dedupes (two consumers share one query)", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      services: [],
      count: 0,
    });
    const { Wrapper } = makeWrapper();
    const a = renderHook(() => useServices(), { wrapper: Wrapper });
    const b = renderHook(() => useServices(), { wrapper: Wrapper });
    await waitFor(() => expect(a.result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledTimes(1);
    expect(b.result.current.data).toEqual({ services: [], count: 0 });
  });

  it("useServices passes tenant param when given", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ services: [], count: 0 });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useServices("acme"), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/services?tenant=acme", expect.any(Object));
  });

  it("useQps scopes cache by window key", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ qps: 1.5 });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useQps(5), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toEqual({ qps: 1.5 }));
    expect(apiFetch).toHaveBeenCalledWith("/qps?window_min=5", expect.any(Object));
  });

  it("useDatasources hits /datasources", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ datasources: [], count: 0 });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useDatasources(), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/datasources", expect.any(Object));
  });

  it("useDashboards hits /dashboards", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ dashboards: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useDashboards(), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/dashboards", expect.any(Object));
  });

  it("useLabels hits /labels", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ keys: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useLabels(), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/labels", expect.any(Object));
  });

  it("useHistogram hits /histogram with bins", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ buckets: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useHistogram("checkout", 10), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith(expect.stringContaining("/histogram?service=checkout"), expect.any(Object));
  });

  it("usePanels hits /dashboards/<id>/panels", async () => {
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ dashboard_id: "d1", panels: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => usePanels("d1"), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/dashboards/d1/panels", expect.any(Object));
  });
});

describe("typed query hooks (continued)", () => {
  beforeEach(() => vi.clearAllMocks());

  it("useService hits /services/<name>", async () => {
    const { useService } = await import("./queries");
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ name: "checkout" });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useService("checkout"), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/services/checkout", expect.any(Object));
  });

  it("useServiceDetail hits /services/<name>/detail", async () => {
    const { useServiceDetail } = await import("./queries");
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ name: "checkout" });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useServiceDetail("checkout"), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/services/checkout/detail", expect.any(Object));
  });

  it("useServiceMap hits /service-map", async () => {
    const { useServiceMap } = await import("./queries");
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ nodes: [], edges: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useServiceMap(), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/service-map", expect.any(Object));
  });

  it("useTrace hits /traces/<id>", async () => {
    const { useTrace } = await import("./queries");
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ trace_id: "t1", spans: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrace("t1"), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith(expect.stringContaining("/traces/t1"), expect.any(Object));
  });

  it("useSeverity hits /severity", async () => {
    const { useSeverity } = await import("./queries");
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ counts: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useSeverity("checkout"), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith(expect.stringContaining("/severity"), expect.any(Object));
  });

  it("useSnapshot hits /snapshot", async () => {
    const { useSnapshot } = await import("./queries");
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ taken_at: "x" });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useSnapshot(), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith("/snapshot", expect.any(Object));
  });

  it("useMetricNames hits /metric-names", async () => {
    const { useMetricNames } = await import("./queries");
    (apiFetch as ReturnType<typeof vi.fn>).mockResolvedValue({ names: [] });
    const { Wrapper } = makeWrapper();
    const { result } = renderHook(() => useMetricNames(20), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(apiFetch).toHaveBeenCalledWith(expect.stringContaining("/metric-names"), expect.any(Object));
  });
});
