// Tiny i18n for the demo-dog dashboard.
//
// Round 24 ships English and Simplified Chinese bundles; the
// interface stays tiny so adding a third locale is one JSON file.
//
// Usage:
//
//   const { t, locale, setLocale } = useI18n();
//   <h1>{t("tenants.title")}</h1>
//
// Missing keys fall back to the key itself so unfinished
// translations do not crash the UI.

export type Locale = "en" | "zh";

type Bundle = Record<string, string>;

const en: Bundle = {
  // Top bar / nav
  "nav.overview": "Overview",
  "nav.explore": "Explore",
  "nav.logs": "Logs",
  "nav.metrics": "Metrics",
  "nav.traces": "Traces",
  "nav.datasources": "Data sources",
  "nav.dashboards": "Dashboards",
  "nav.ingest": "Ingest demo",
  "nav.live": "Live",
  "nav.serviceMap": "Service map",
  "nav.alerts": "Alerts",
  "nav.tenants": "Tenants",

  // Status
  "status.healthy": "Healthy",
  "status.degraded": "Degraded",
  "status.down": "Down",
  "status.loading": "Loading...",

  // Buttons / forms
  "button.refresh": "Refresh",
  "button.cancel": "Cancel",
  "button.save": "Save",
  "button.create": "Create",
  "button.delete": "Delete",
  "button.mint": "Mint",
  "button.search": "Search",
  "button.replay": "Replay",

  // Tenants page
  "tenants.title": "Tenants",
  "tenants.subtitle":
    "Each tenant gets its own slice of logs, metrics, and traces. API keys minted here are bound to a tenant and cannot read data from other tenants.",
  "tenants.createTitle": "Create tenant",
  "tenants.idPlaceholder": "acme",
  "tenants.namePlaceholder": "Acme Corp",
  "tenants.mintTitle": "Mint API key",
  "tenants.labelPlaceholder": "checkout",
  "tenants.mintOnceHint": "copy now, this is the only time the plaintext is shown",
  "tenants.empty": "No tenants yet. Use the form above to add one.",
  "tenants.active": "active",
  "tenants.disabled": "disabled",
  "tenants.createdAt": "Created",

  // Errors
  "error.unauthorized": "You are not authorized to view this page.",
  "error.network": "Network error. Please check your connection.",
  "error.unknown": "Something went wrong.",
};

const zh: Bundle = {
  "nav.overview": "概览",
  "nav.explore": "探索",
  "nav.logs": "日志",
  "nav.metrics": "指标",
  "nav.traces": "链路",
  "nav.datasources": "数据源",
  "nav.dashboards": "仪表盘",
  "nav.ingest": "数据接入演示",
  "nav.live": "实时",
  "nav.serviceMap": "服务拓扑",
  "nav.alerts": "告警",
  "nav.tenants": "租户",

  "status.healthy": "健康",
  "status.degraded": "降级",
  "status.down": "宕机",
  "status.loading": "加载中...",

  "button.refresh": "刷新",
  "button.cancel": "取消",
  "button.save": "保存",
  "button.create": "创建",
  "button.delete": "删除",
  "button.mint": "签发",
  "button.search": "搜索",
  "button.replay": "重放",

  "tenants.title": "租户管理",
  "tenants.subtitle":
    "每个租户拥有独立的日志、指标、链路切片。 在此处签发的 API key 被绑定到具体租户， 不能读取其他租户的数据。",
  "tenants.createTitle": "创建租户",
  "tenants.idPlaceholder": "acme",
  "tenants.namePlaceholder": "Acme 公司",
  "tenants.mintTitle": "签发 API key",
  "tenants.labelPlaceholder": "checkout",
  "tenants.mintOnceHint": "请立即复制，明文只会显示一次",
  "tenants.empty": "暂无租户。请使用上方表单添加。",
  "tenants.active": "启用",
  "tenants.disabled": "停用",
  "tenants.createdAt": "创建时间",

  "error.unauthorized": "您没有权限查看此页面。",
  "error.network": "网络错误，请检查连接。",
  "error.unknown": "未知错误。",
};

const BUNDLES: Record<Locale, Bundle> = { en, zh };

export const LOCALES: Array<{ id: Locale; label: string }> = [
  { id: "en", label: "English" },
  { id: "zh", label: "简体中文" },
];

export function translate(locale: Locale, key: string): string {
  return BUNDLES[locale]?.[key] ?? BUNDLES.en[key] ?? key;
}

export function listLocales() {
  return Object.keys(BUNDLES);
}
