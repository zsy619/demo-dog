import { useEffect, useRef, useState } from "react";
import type { Page } from "@/App";
import { useStreamStatus } from "@/hooks/useStream";
import { useAuth } from "@/hooks/useAuth";
import { useHealth, useServices } from "@/hooks/queries";
import { useI18n } from "@/i18n/I18nProvider";
import { LOCALES } from "@/i18n";
import CountUp from "./anim/CountUp";
import Pulse from "./anim/Pulse";
import Glitch from "./anim/Glitch";
import TrueFocus from "./anim/TrueFocus";
import BlurText from "./anim/BlurText";

const TITLES: Record<Page, string> = {
  "overview": "Service Overview",
  "explore": "Explore",
  "logs": "Logs",
  "metrics": "Metrics",
  "traces": "Distributed Traces",
  "datasources": "Data sources",
  "dashboards": "Dashboards",
  "ingest": "Ingest demo",
  "live": "Live tail",
  "service-map": "Service map",
  "service-detail": "Service",
  alerts: "Alerts",
  tenants: "Tenants",
  audit: "Audit log",
  probes: "Probes",
  webhooks: "Webhooks",
  retention: "Retention",
  slos: "SLO budgets",
  replica: "Replica state",
  "admin-keys": "Admin keys",
};

interface Props {
  page: Page;
  onPageChange: (p: Page) => void;
  onOpenLogin?: () => void;
}

interface CounterSnapshot {
  logs: number;
  metrics: number;
  spans: number;
  ts: number;
}

export default function TopBar({ page, onOpenLogin }: Props) {
  const wsStatus = useStreamStatus();
  const auth = useAuth();
  const healthQuery = useHealth();
  const servicesQuery = useServices();
  const health = healthQuery.data ?? null;
  const error = healthQuery.error
    ? String(healthQuery.error.message ?? healthQuery.error)
    : null;
  const [now, setNow] = useState(() => new Date());
  const prevRef = useRef<CounterSnapshot | null>(null);
  const [rate, setRate] = useState<{ logs: number; metrics: number; spans: number }>({
    logs: 0,
    metrics: 0,
    spans: 0,
  });
  // Average error rate across services drives the global alert chip in the
  // TopBar. The query refetches itself on focus + every 8s; no manual
  // polling needed.
  const svcs = servicesQuery.data?.services ?? [];
  const avgErr =
    svcs.length > 0 ? svcs.reduce((a, s) => a + s.error_rate, 0) / svcs.length : 0;

  // Derive throughput each time the health payload updates. We keep the
  // previous snapshot in a ref so the rate calculation has a baseline.
  useEffect(() => {
    if (!health) return;
    const prev = prevRef.current;
    if (prev) {
      const now2 = Date.now();
      const dt = Math.max(1, now2 - prev.ts) / 1000;
      setRate({
        logs: (health.engine.logs_accepted - prev.logs) / dt,
        metrics: (health.engine.metrics_accepted - prev.metrics) / dt,
        spans: (health.engine.spans_accepted - prev.spans) / dt,
      });
    }
    prevRef.current = {
      logs: health.engine.logs_accepted,
      metrics: health.engine.metrics_accepted,
      spans: health.engine.spans_accepted,
      ts: Date.now(),
    };
  }, [health]);

  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const ok = health?.status === "ok";

  return (
    <header className="h-12 shrink-0 border-b border-grafana-border bg-grafana-panel flex items-center px-4 gap-3">
      <div className="text-[13px] font-semibold whitespace-nowrap">
        <BlurText text={TITLES[page]} duration={350} stagger={22} />
      </div>
      <div className="text-[11px] text-grafana-muted whitespace-nowrap">/ demo-dog</div>
      <div className="flex-1" />

      <div className="flex items-center gap-2 text-[11px]">
        <div
          className={`flex items-center gap-1.5 px-2 py-1 rounded border ${
            ok
              ? "border-grafana-ok/30 bg-grafana-ok/10 text-grafana-ok"
              : "border-grafana-err/40 bg-grafana-err/10 text-grafana-err"
          }`}
          title={ok ? `uptime ${health?.uptime ?? "--"}` : error ?? ""}
        >
          {ok ? (
            <Pulse color="#10b981" size={8} />
          ) : (
            <Pulse color="#ef4444" size={8} />
          )}
          {ok ? "engine online" : "engine offline"}
        </div>

        <div
          className={`flex items-center gap-1.5 px-2 py-1 rounded border ${
            wsStatus === "open"
              ? "border-grafana-accent/30 bg-grafana-accent/10 text-grafana-accent"
              : wsStatus === "connecting"
              ? "border-grafana-warn/40 bg-grafana-warn/10 text-grafana-warn"
              : "border-grafana-err/40 bg-grafana-err/10 text-grafana-err"
          }`}
          title={`WebSocket ${wsStatus}`}
        >
          {wsStatus === "open" ? (
            <Pulse color="#a855f7" size={8} />
          ) : wsStatus === "connecting" ? (
            <Pulse color="#f59e0b" size={8} />
          ) : (
            <Pulse color="#ef4444" size={8} />
          )}
          ws {wsStatus}
        </div>

        {/* Global error-rate alert. Only renders when the average across
            services exceeds 1%; glitch text when it crosses 5%. The chip is
            deliberately loud because the user might be on Logs/Metrics and
            not looking at the Overview tiles. */}
        {avgErr > 0.01 && (
          <div
            className="flex items-center gap-1.5 px-2 py-1 rounded border border-grafana-err/40 bg-grafana-err/10 text-grafana-err"
            title={`average error rate across all services: ${(avgErr * 100).toFixed(1)}%`}
          >
            <Pulse color="#ef4444" size={8} />
            <span className="font-mono tabular-nums">
              {(avgErr * 100).toFixed(1)}%
            </span>
            {avgErr > 0.05 ? (
              <Glitch>
                <span className="font-semibold uppercase tracking-wider text-[10px]">
                  sev
                </span>
              </Glitch>
            ) : (
              <span className="font-semibold uppercase tracking-wider text-[10px]">
                err
              </span>
            )}
          </div>
        )}

        {health && (
          <>
            <div
              className="px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 flex items-center gap-2"
              title="hot vs cold tier occupancy"
            >
              <span className="text-grafana-muted">tiers</span>
              <TierBar
                hot={health.engine.hot_logs + health.engine.hot_metrics + health.engine.hot_spans}
                cold={health.engine.cold_logs + health.engine.cold_metrics + health.engine.cold_spans}
              />
              <span className="font-mono text-grafana-text">
                {health.engine.hot_logs + health.engine.hot_metrics + health.engine.hot_spans}
              </span>
              <span className="text-grafana-muted">hot</span>
            </div>

            <div
              className="px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 flex items-center gap-1.5"
              title="active services"
            >
              <span className="text-grafana-muted">svc</span>
              <CountUp
                value={health.engine.services}
                className="text-grafana-text font-semibold tabular-nums"
              />
            </div>

            <div className="px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 flex items-center gap-2 font-mono text-[11px]">
              <Pulse color="#a855f7" size={6} />
              <span title="logs/sec" className="flex items-center gap-1">
                <span className="text-grafana-muted">L</span>
                <CountUp
                  value={rate.logs}
                  format={(n) => n.toFixed(1)}
                  className="text-grafana-text font-semibold tabular-nums"
                />
              </span>
              <span title="metrics/sec" className="flex items-center gap-1">
                <span className="text-grafana-muted">M</span>
                <CountUp
                  value={rate.metrics}
                  format={(n) => n.toFixed(1)}
                  className="text-grafana-text font-semibold tabular-nums"
                />
              </span>
              <span title="spans/sec" className="flex items-center gap-1">
                <span className="text-grafana-muted">S</span>
                <CountUp
                  value={rate.spans}
                  format={(n) => n.toFixed(1)}
                  className="text-grafana-text font-semibold tabular-nums"
                />
              </span>
              <span className="text-grafana-muted">/s</span>
            </div>

            <div className="px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 flex items-center gap-2 text-grafana-muted">
              <span title="total logs">
                <CountUp
                  value={health.engine.logs_accepted}
                  format={(n) => Math.round(n).toLocaleString()}
                  className="text-grafana-text font-semibold tabular-nums"
                />{" "}
                logs
              </span>
              <span className="text-grafana-border">·</span>
              <span title="total metrics">
                <CountUp
                  value={health.engine.metrics_accepted}
                  format={(n) => Math.round(n).toLocaleString()}
                  className="text-grafana-text font-semibold tabular-nums"
                />{" "}
                metrics
              </span>
              <span className="text-grafana-border">·</span>
              <span title="total spans">
                <CountUp
                  value={health.engine.spans_accepted}
                  format={(n) => Math.round(n).toLocaleString()}
                  className="text-grafana-text font-semibold tabular-nums"
                />{" "}
                spans
              </span>
            </div>

            <div
              className="px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 text-grafana-muted"
              title="queries served"
            >
              queries{" "}
              <CountUp
                value={health.engine.queries_served}
                format={(n) => Math.round(n).toLocaleString()}
                className="text-grafana-text font-semibold tabular-nums"
              />
            </div>
          </>
        )}

        <div className="px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 font-mono text-grafana-text tabular-nums">
          {now.toLocaleTimeString()}
        </div>

        <LocalePicker />

        <button
          type="button"
          onClick={() => onOpenLogin?.()}
          className={
            "px-2 py-1 rounded border text-xs flex items-center gap-1 " +
            (auth.isAuthed
              ? "border-emerald-700 bg-emerald-900/30 text-emerald-300 hover:bg-emerald-900/50"
              : "border-amber-700 bg-amber-900/30 text-amber-300 hover:bg-amber-900/50")
          }
          aria-label={auth.isAuthed ? "Authenticated" : "Not authenticated"}
          title={
            auth.isAuthed
              ? `Connected (key ending …${auth.apiKey.slice(-4)})`
              : "Click to enter API key"
          }
        >
          <span
            className={
              "inline-block w-1.5 h-1.5 rounded-full " +
              (auth.isAuthed ? "bg-emerald-400" : "bg-amber-400")
            }
            aria-hidden="true"
          />
          {auth.isAuthed ? "Auth" : "Login"}
        </button>
      </div>
    </header>
  );
}

function TierBar({ hot, cold }: { hot: number; cold: number }) {
  const total = Math.max(1, hot + cold);
  const hotPct = (hot / total) * 100;
  return (
    <div className="relative h-3 w-16 rounded overflow-hidden bg-grafana-elev border border-grafana-border">
      <div
        className="absolute inset-y-0 left-0 bg-grafana-accent/70"
        style={{ width: `${hotPct}%` }}
      />
      <div
        className="absolute inset-y-0 right-0 bg-grafana-err/40"
        style={{ width: `${100 - hotPct}%` }}
      />
    </div>
  );
}

function LocalePicker() {
  const { locale, setLocale } = useI18n();
  return (
    <label
      className="px-2 py-1 rounded border border-grafana-border bg-grafana-elev/40 text-grafana-text text-[11px] flex items-center gap-1"
      aria-label="Language selector"
    >
      <span aria-hidden="true">🌐</span>
      <select
        value={locale}
        onChange={(e) => setLocale(e.target.value as "en" | "zh")}
        className="bg-transparent outline-none cursor-pointer"
      >
        {LOCALES.map((l) => (
          <option key={l.id} value={l.id}>{l.label}</option>
        ))}
      </select>
    </label>
  );
}
