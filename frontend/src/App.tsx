import { useCallback, useEffect, useMemo } from "react";
import Sidebar from "@/components/Sidebar";
import TopBar from "@/components/TopBar";
import Overview from "@/pages/Overview";
import Explore from "@/pages/Explore";
import Logs from "@/pages/Logs";
import Metrics from "@/pages/Metrics";
import Traces from "@/pages/Traces";
import DataSources from "@/pages/DataSources";
import Dashboards from "@/pages/Dashboards";
import IngestDemo from "@/pages/IngestDemo";
import Live from "@/pages/Live";
import ServiceMapPage from "@/pages/ServiceMapPage";
import ServiceDetailPage from "@/pages/ServiceDetailPage";
import CommandPalette from "@/components/CommandPalette";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { ToastHost } from "@/components/Toast";
import { useRoute } from "@/lib/router";

export type Page =
  | "overview"
  | "explore"
  | "logs"
  | "metrics"
  | "traces"
  | "datasources"
  | "dashboards"
  | "ingest"
  | "live"
  | "service-map"
  | "service-detail";

const VALID_PAGES: Page[] = [
  "overview",
  "explore",
  "logs",
  "metrics",
  "traces",
  "datasources",
  "dashboards",
  "ingest",
  "live",
  "service-map",
  "service-detail",
];

function isValid(p: string): p is Page {
  return (VALID_PAGES as string[]).includes(p);
}

export default function App() {
  const [page, params, navigate] = useRoute();
  const safePage: Page = isValid(page) ? page : "overview";

  const service = params.get("service") ?? "";
  const signal = (params.get("signal") as "logs" | "metrics" | "traces") ?? "logs";
  const traceId = params.get("trace_id") ?? "";

  // Top-level keyboard shortcuts (vim-ish): g l / g m / g t / g o / g e / g d / g s
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      if (e.key === "g") {
        const next = (e2: KeyboardEvent) => {
          window.removeEventListener("keydown", next);
          const p =
            (e2.key.toLowerCase() === "l" && "logs") ||
            (e2.key.toLowerCase() === "m" && "metrics") ||
            (e2.key.toLowerCase() === "t" && "traces") ||
            (e2.key.toLowerCase() === "o" && "overview") ||
            (e2.key.toLowerCase() === "e" && "explore") ||
            (e2.key.toLowerCase() === "d" && "dashboards") ||
            (e2.key.toLowerCase() === "s" && "service-map") ||
            (e2.key.toLowerCase() === "v" && "service-detail") ||
            null;
          if (p) navigate(p);
        };
        window.addEventListener("keydown", next);
        window.setTimeout(() => window.removeEventListener("keydown", next), 1000);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [navigate]);

  const setService = useCallback(
    (s: string) => {
      const next = new URLSearchParams(params);
      if (s) next.set("service", s);
      else next.delete("service");
      navigate(safePage, next);
    },
    [params, navigate, safePage]
  );

  const setSignal = useCallback(
    (s: "logs" | "metrics" | "traces") => {
      const next = new URLSearchParams(params);
      next.set("signal", s);
      navigate(safePage, next);
    },
    [params, navigate, safePage]
  );

  const setPage = useCallback((p: Page) => navigate(p, params), [navigate, params]);

  const main = useMemo(() => {
    switch (safePage) {
      case "overview":
        return (
          <Overview
            onServiceSelect={(s) => {
              const next = new URLSearchParams();
              next.set("service", s);
              navigate("explore", next);
            }}
            onServiceDrillIn={(s) => {
              const next = new URLSearchParams();
              next.set("service", s);
              navigate("service-detail", next);
            }}
          />
        );
      case "explore":
        return (
          <Explore
            service={service}
            signal={signal}
            onSignalChange={setSignal}
            onServiceChange={setService}
            onFilterChange={(k, v) => {
              const next = new URLSearchParams(params);
              next.set(k, v);
              navigate(safePage, next);
            }}
          />
        );
      case "logs":
        return (
          <Logs
            service={service}
            onServiceChange={setService}
            onOpenTrace={(tid) => {
              const next = new URLSearchParams();
              next.set("trace_id", tid);
              navigate("traces", next);
            }}
            initialTraceId={traceId}
          />
        );
      case "metrics":
        return <Metrics service={service} onServiceChange={setService} />;
      case "traces":
        return (
          <Traces
            service={service}
            onServiceChange={setService}
            initialTraceId={traceId}
          />
        );
      case "datasources":
        return <DataSources />;
      case "dashboards":
        return (
          <Dashboards
            onOpen={(id) => {
              const next = new URLSearchParams();
              next.set("dashboard", id);
              navigate("dashboards", next);
            }}
          />
        );
      case "ingest":
        return <IngestDemo />;
      case "live":
        return <Live service={service} onServiceChange={setService} />;
      case "service-map":
        return <ServiceMapPage />;
      case "service-detail":
        return (
          <ServiceDetailPage
            service={service}
            onNavigate={(p, p2) => navigate(p, new URLSearchParams(p2))}
          />
        );
      default:
        return null;
    }
  }, [safePage, service, signal, traceId, navigate, setService, setSignal]);

  return (
    <div className="flex h-screen w-screen bg-grafana-bg text-grafana-text">
      <Sidebar
        page={safePage}
        onPageChange={(p) => navigate(p)}
        service={service}
        onServiceChange={setService}
      />
      <div className="flex-1 flex flex-col min-w-0">
        <TopBar page={safePage} onPageChange={setPage} />
        <main className="flex-1 overflow-y-auto scrollbar-thin">
          <ErrorBoundary key={safePage}>{main}</ErrorBoundary>
        </main>
      </div>
      <ToastHost />
      <CommandPalette page={safePage} navigate={navigate} onServiceChange={setService} />
    </div>
  );
}
