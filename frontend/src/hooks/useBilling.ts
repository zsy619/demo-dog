import { useCallback, useEffect, useState } from "react";

import { api } from "@/lib/api";
import type {
  BillingUsage,
  BillingPeriodTotal,
} from "@/types/api";

export interface BillingRow {
  tenant: string;
  metric: string;
  period: string; // YYYY-MM, "" = 全周期
  value: number;
  updatedAt?: string;
}

export interface UseBillingResult {
  // 总览(全租户全指标):扁平 PeriodTotal 列表。
  rows: BillingPeriodTotal[];
  // 按当前 tenant 筛选后的 Usage 视图。
  usage: BillingUsage[];
  loading: boolean;
  error: Error | null;
  reload: () => Promise<void>;
}

// useBilling 拉取 /api/v1/billing/usage 的全表结果,并
// 按当前 tenant 过滤出 Usage 视图。后端返回所有 (period, metric)
// 组合的 PeriodTotal,前端把它们聚合成 UI 友好的二维表。
//
// 失败时把错误抛给 UI;loading=true 期间 UI 可以渲染骨架。
//
// 用法:
//   const { rows, usage, loading, error, reload } = useBilling(tenant);
export function useBilling(tenant: string): UseBillingResult {
  const [rows, setRows] = useState<BillingPeriodTotal[]>([]);
  const [usage, setUsage] = useState<BillingUsage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const all = await api.billingUsageAll();
      const safeRows = Array.isArray(all.rows) ? all.rows : [];
      setRows(safeRows);
      if (tenant) {
        const u = await api.billingUsageTenant(tenant);
        setUsage(Array.isArray(u.usage) ? u.usage : []);
      } else {
        setUsage([]);
      }
    } catch (e) {
      setError(e as Error);
      setRows([]);
      setUsage([]);
    } finally {
      setLoading(false);
    }
  }, [tenant]);

  useEffect(() => {
    reload();
    const t = setInterval(reload, 15_000);
    return () => clearInterval(t);
  }, [reload]);

  return { rows, usage, loading, error, reload };
}

// flattenUsageToRows 把 Usage[] 拍平成 BillingRow,适合在表格里
// 一次性渲染所有 (tenant, metric, period) 行。
export function flattenUsageToRows(
  tenant: string,
  usage: BillingUsage[]
): BillingRow[] {
  const out: BillingRow[] = [];
  for (const u of usage) {
    for (const [period, value] of Object.entries(u.periods)) {
      out.push({
        tenant: tenant || u.tenant,
        metric: u.metric,
        period,
        value,
      });
    }
  }
  return out;
}

// currentPeriod 返回 "YYYY-MM" 字符串,用于 UI 默认值。
export function currentPeriod(now: Date = new Date()): string {
  const y = now.getUTCFullYear();
  const m = String(now.getUTCMonth() + 1).padStart(2, "0");
  return `${y}-${m}`;
}
