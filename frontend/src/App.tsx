import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from "react";
import Sidebar from "@/components/Sidebar";
import TopBar from "@/components/TopBar";
import CommandPalette from "@/components/CommandPalette";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { ToastHost } from "@/components/Toast";
import { useRoute } from "@/lib/router";
import { LoginModal } from "@/components/LoginModal";
import { useAuth } from "@/hooks/useAuth";

// Lazy-load every page. The shell (TopBar + Sidebar + route chrome)
// ships in the entry chunk; each page is its own dynamic chunk so a
// user who never opens the Logs page never downloads it.
const Overview = lazy(() => import("@/pages/Overview"));
const Explore = lazy(() => import("@/pages/Explore"));
const Logs = lazy(() => import("@/pages/Logs"));
const Metrics = lazy(() => import("@/pages/Metrics"));
const Traces = lazy(() => import("@/pages/Traces"));
const DataSources = lazy(() => import("@/pages/DataSources"));
const Dashboards = lazy(() => import("@/pages/Dashboards"));
const IngestDemo = lazy(() => import("@/pages/IngestDemo"));
const Live = lazy(() => import("@/pages/Live"));
const ServiceMapPage = lazy(() => import("@/pages/ServiceMapPage"));
const ServiceDetailPage = lazy(() => import("@/pages/ServiceDetailPage"));
const Alerts = lazy(() => import("@/pages/Alerts"));
const Tenants = lazy(() => import("@/pages/Tenants"));

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
  | "service-detail"
  | "alerts"
  | "tenants";

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
  "alerts",
  "tenants",
];

function isValid(p: string): p is Page {
  return (VALID_PAGES as string[]).includes(p);
}

function PageFallback() {
  // Skeleton placeholder so the layout does not jump while a chunk
  // is loading. The bar is intentionally subtle.
  return (
    <div className="h-full w-full flex items-center justify-center text-grafana-muted text-xs">
      <div className="animate-pulse">Loading…</div>
    </div>
  );
}

export default function App() {
  const [page, params, navigate] = useRoute();
  const safePage: Page = isValid(page) ? page : "overview";
  const auth = useAuth();
  const [showLogin, setShowLogin] = useState(false);
  const [loginError, setLoginError] = useState<string | undefined>(
    undefined
  );

  const service = params.get("service") ?? "";
  const signal = (params.get("signal") as "logs" | "metrics" | "traces") ?? "logs";
  const traceId = params.get("trace_id") ?? "";

  // Listen for global 401 events from the fetch wrapper. When the
  // collector rejects a key we surface a login modal so the user
  // can re-authenticate without leaving the page they were on.
  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<{ status: number }>).detail;
      if (detail?.status === 401) {
        setLoginError("API key rejected by collector (401)");
        setShowLogin(true);
      }
    };
    window.addEventListener("dog:auth-error", handler);
    return () => window.removeEventListener("dog:auth-error", handler);
  }, []);

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
            (e2.key.toLowerCase() === "a" && "alerts") ||
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
      case "alerts":
        return <Alerts />;
      case "tenants":
        return <Tenants />;
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
        <TopBar
          page={safePage}
          onPageChange={setPage}
          onOpenLogin={() => {
            setLoginError(undefined);
            setShowLogin(true);
          }}
        />
        <main className="flex-1 overflow-y-auto scrollbar-thin">
          <ErrorBoundary key={safePage}>
            <Suspense fallback={<PageFallback />}>{main}</Suspense>
          </ErrorBoundary>
        </main>
      </div>
      <ToastHost />
      <CommandPalette page={safePage} navigate={navigate} onServiceChange={setService} />
      {showLogin && (
        <LoginModal
          onClose={() => {
            setShowLogin(false);
            setLoginError(undefined);
          }}
          errorMessage={loginError}
        />
      )}
    </div>
  );
}
