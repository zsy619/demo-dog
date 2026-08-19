import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { ServiceSummary } from "@/types/api";
import type { Page } from "@/App";
import Sparkline from "@/components/Sparkline";
import { useAuth } from "@/hooks/useAuth";

interface SidebarProps {
  page: Page;
  onPageChange: (p: Page) => void;
  service: string;
  onServiceChange: (s: string) => void;
}

const PRIMARY_ITEMS: Array<{ id: Page; label: string; icon: string }> = [
  { id: "overview", label: "Overview", icon: "◧" },
  { id: "service-detail", label: "Service", icon: "◎" },
  { id: "explore", label: "Explore", icon: "⌘" },
  { id: "live", label: "Live tail", icon: "⚡" },
  { id: "dashboards", label: "Dashboards", icon: "◧" },
  { id: "service-map", label: "Service map", icon: "⇄" },
];

const SIGNAL_ITEMS: Array<{ id: Page; label: string; icon: string }> = [
  { id: "logs", label: "Logs", icon: "☰" },
  { id: "metrics", label: "Metrics", icon: "↗" },
  { id: "traces", label: "Traces", icon: "→" },
];

const ADMIN_ITEMS: Array<{ id: Page; label: string; icon: string }> = [
  { id: "datasources", label: "Data sources", icon: "▤" },
  { id: "ingest", label: "Ingest demo", icon: "⇄" },
  { id: "alerts", label: "Alerts", icon: "⚑" },
  { id: "tenants", label: "Tenants", icon: "⌹" },
  { id: "audit", label: "Audit", icon: "◉" },
  { id: "probes", label: "Probes", icon: "↗" },
  { id: "webhooks", label: "Webhooks", icon: "⚡" },
  { id: "retention", label: "Retention", icon: "♻" },
  { id: "slos", label: "SLOs", icon: "%" },
  { id: "replica", label: "Replica", icon: "⇄" },
  { id: "admin-keys", label: "Admin keys", icon: "⚿" },
];

function NavItem({
  active,
  label,
  icon,
  onClick,
  hint,
}: {
  active: boolean;
  label: string;
  icon: string;
  onClick: () => void;
  hint?: string;
}) {
  return (
    <button
      onClick={onClick}
      title={hint}
      className={`w-full text-left px-3 py-1.5 rounded-md flex items-center gap-2 text-sm transition-colors ${
        active
          ? "bg-grafana-accent/15 text-grafana-accent"
          : "hover:bg-grafana-elev text-grafana-muted hover:text-grafana-text"
      }`}
    >
      <span className="font-mono text-[13px] w-4 text-center opacity-80">
        {icon}
      </span>
      <span className="flex-1">{label}</span>
      {hint && <span className="text-[10px] text-grafana-muted">{hint}</span>}
    </button>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="mt-4">
      <div className="px-3 mb-1 text-[11px] uppercase tracking-wider text-grafana-muted">
        {title}
      </div>
      <div className="space-y-0.5">{children}</div>
    </div>
  );
}

function ServiceRow({
  svc,
  active,
  onClick,
  qpsSpark,
}: {
  svc: ServiceSummary;
  active: boolean;
  onClick: () => void;
  qpsSpark: number[];
}) {
  const err = svc.error_rate * 100;
  const dot =
    err > 5
      ? "bg-grafana-err"
      : err > 1
      ? "bg-grafana-warn"
      : "bg-grafana-ok";
  return (
    <button
      onClick={onClick}
      className={`w-full text-left px-3 py-1.5 rounded-md text-sm flex items-center gap-2 transition-colors ${
        active
          ? "bg-grafana-elev text-grafana-text"
          : "hover:bg-grafana-elev text-grafana-muted hover:text-grafana-text"
      }`}
    >
      <span className="flex items-center gap-2 truncate flex-1 min-w-0">
        <span className={`w-1.5 h-1.5 rounded-full ${dot}`} />
        <span className="truncate">{svc.name}</span>
      </span>
      <Sparkline
        points={qpsSpark}
        width={48}
        height={14}
        color={err > 5 ? "#ef4444" : err > 1 ? "#f59e0b" : "#3b82f6"}
      />
      <span className="text-[10px] text-grafana-muted font-mono w-12 text-right">
        {svc.p99_ms.toFixed(0)}ms
      </span>
    </button>
  );
}

function TenantSwitcher() {
  const { tenantId, setTenantId } = useAuth();
  return (
    <div className="px-3 pb-2 border-b border-grafana-border">
      <label className="text-[10px] uppercase tracking-wider text-grafana-muted block mb-1">
        Tenant
      </label>
      <input
        value={tenantId}
        onChange={(e) => setTenantId(e.target.value)}
        placeholder="all tenants"
        spellCheck={false}
        autoComplete="off"
        className="w-full bg-grafana-elev border border-grafana-border rounded px-2 py-1 text-xs focus:outline-none focus:border-grafana-blue"
      />
      <p className="mt-1 text-[10px] text-grafana-muted leading-tight">
        Sent as{" "}
        <code className="font-mono">X-Tenant-Id</code> on every API
        request and{" "}
        <code className="font-mono">?tenant=</code> on read queries.
        Empty = no tenant filter.
      </p>
    </div>
  );
}

export default function Sidebar({
  page,
  onPageChange,
  service,
  onServiceChange,
}: SidebarProps) {
  const auth = useAuth();
  const [services, setServices] = useState<ServiceSummary[]>([]);
  const [qpsByService, setQpsByService] = useState<Record<string, number[]>>({});
  const [filter, setFilter] = useState("");

  // Re-load the service list whenever the tenant changes. The hook
  // dependency list intentionally only re-runs on tenant change so
  // we do not thrash the collector when nothing relevant shifted.
  useEffect(() => {
    let cancelled = false;
    const load = () =>
      api
        .services(auth.tenantId || undefined)
        .then((r) => {
          if (!cancelled) setServices(r.services);
        })
        .catch(() => {});
    load();
    const id = window.setInterval(load, 5000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [auth.tenantId]);

  // Fetch the per-service QPS series and turn it into a tiny ring buffer of
  // recent samples so each sidebar row can show a sparkline.
  useEffect(() => {
    let cancelled = false;
    const load = () =>
      api
        .qps(5)
        .then((r) => {
          if (cancelled) return;
          // Drop null / non-numeric points before the sparkline sees them.
          const series = r.series.map((s) => ({
            name: s.service,
            points: (s.points ?? [])
              .filter((p) => p && typeof p.value === "number")
              .map((p) => p.value),
          }));
          setQpsByService((prev) => {
            const next: Record<string, number[]> = {};
            for (const row of series) {
              const last = row.points[row.points.length - 1] ?? 0;
              const p = prev[row.name] ?? [];
              const updated = [...p, last].slice(-20);
              next[row.name] = updated;
            }
            return next;
          });
        })
        .catch(() => {});
    load();
    const id = window.setInterval(load, 4000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  const filtered = useMemo(
    () =>
      filter
        ? services.filter((s) =>
            s.name.toLowerCase().includes(filter.toLowerCase())
          )
        : services,
    [services, filter]
  );

  return (
    <aside className="w-60 shrink-0 bg-grafana-panel border-r border-grafana-border flex flex-col">
      <div className="px-4 py-3 border-b border-grafana-border flex items-center gap-2">
        <div className="w-7 h-7 rounded-md bg-gradient-to-br from-grafana-accent to-grafana-accent2 flex items-center justify-center font-bold text-white text-xs">
          DOG
        </div>
        <div>
          <div className="text-[13px] font-semibold leading-tight">DOG</div>
          <div className="text-[11px] text-grafana-muted leading-tight">
            Doris + OTel + Grafana
          </div>
        </div>
      </div>

      <TenantSwitcher />

      <div className="flex-1 overflow-y-auto scrollbar-thin py-3">
        <Section title="General">
          {PRIMARY_ITEMS.map((it) => (
            <NavItem
              key={it.id}
              active={page === it.id}
              label={it.label}
              icon={it.icon}
              onClick={() => onPageChange(it.id)}
            />
          ))}
        </Section>

        <Section title="Signals">
          {SIGNAL_ITEMS.map((it) => (
            <NavItem
              key={it.id}
              active={page === it.id}
              label={it.label}
              icon={it.icon}
              onClick={() => onPageChange(it.id)}
            />
          ))}
        </Section>

        <Section title="Admin">
          {ADMIN_ITEMS.map((it) => (
            <NavItem
              key={it.id}
              active={page === it.id}
              label={it.label}
              icon={it.icon}
              onClick={() => onPageChange(it.id)}
            />
          ))}
        </Section>

        <Section title={`Services (${services.length})`}>
          {services.length > 0 && (
            <div className="px-3 pb-1 text-[10px] text-grafana-muted flex items-center gap-2">
              <span title="healthy services (< 1% error rate)">
                <span className="inline-block w-1.5 h-1.5 rounded-full bg-grafana-ok mr-0.5" />
                {services.filter((s) => s.error_rate < 0.01).length}
              </span>
              <span title="degraded (1–5% error rate)">
                <span className="inline-block w-1.5 h-1.5 rounded-full bg-grafana-warn mr-0.5" />
                {
                  services.filter((s) => s.error_rate >= 0.01 && s.error_rate < 0.05)
                    .length
                }
              </span>
              <span title="critical (> 5% error rate)">
                <span className="inline-block w-1.5 h-1.5 rounded-full bg-grafana-err mr-0.5" />
                {services.filter((s) => s.error_rate >= 0.05).length}
              </span>
            </div>
          )}
          {services.length > 4 && (
            <div className="px-2 mb-1">
              <input
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder="Filter…"
                className="w-full bg-grafana-elev border border-grafana-border rounded px-2 py-1 text-xs focus:outline-none focus:border-grafana-blue"
              />
            </div>
          )}
          <div className="space-y-0.5">
            {filtered.length === 0 && services.length > 0 && (
              <div className="px-3 py-2 text-[12px] text-grafana-muted italic">
                No services match.
              </div>
            )}
            {services.length === 0 && (
              <div className="px-3 py-2 text-[12px] text-grafana-muted italic">
                No services yet. Try /api/seed.
              </div>
            )}
            {filtered.map((s) => (
              <ServiceRow
                key={s.name}
                svc={s}
                active={service === s.name}
                qpsSpark={qpsByService[s.name] ?? []}
                onClick={() => {
                  onServiceChange(s.name);
                  onPageChange("service-detail");
                }}
              />
            ))}
          </div>
        </Section>
      </div>

      <div className="px-3 py-2 border-t border-grafana-border text-[11px] text-grafana-muted flex justify-between">
        <span>v0.1.0 demo</span>
        <span className="text-grafana-ok">● live</span>
      </div>
    </aside>
  );
}
