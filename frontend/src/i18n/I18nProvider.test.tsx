import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import type { ReactNode } from "react";
import { I18nProvider, useI18n } from "./I18nProvider";

function wrapper({ children }: { children: ReactNode }) {
  return <I18nProvider>{children}</I18nProvider>;
}

describe("I18nProvider", () => {
  it("translates known keys in English", () => {
    const { result } = renderHook(() => useI18n(), { wrapper });
    expect(result.current.t("button.refresh")).toBe("Refresh");
    expect(result.current.t("tenants.title")).toBe("Tenants");
  });

  it("falls back to the key when translation missing", () => {
    const { result } = renderHook(() => useI18n(), { wrapper });
    expect(result.current.t("unknown.key")).toBe("unknown.key");
  });

  it("switches to Chinese and back", () => {
    const { result } = renderHook(() => useI18n(), { wrapper });
    act(() => result.current.setLocale("zh"));
    expect(result.current.t("button.refresh")).toBe("刷新");
    expect(result.current.t("tenants.title")).toBe("租户管理");
    act(() => result.current.setLocale("en"));
    expect(result.current.t("button.refresh")).toBe("Refresh");
  });

  it("updates document.documentElement.lang", () => {
    const { result } = renderHook(() => useI18n(), { wrapper });
    act(() => result.current.setLocale("zh"));
    expect(document.documentElement.lang).toBe("zh");
    act(() => result.current.setLocale("en"));
    expect(document.documentElement.lang).toBe("en");
  });
});
