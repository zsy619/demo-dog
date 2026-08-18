// React context that exposes the active locale + a translator.
//
// Persists to localStorage so the user's choice sticks across
// reloads. Defaults to the navigator language when no choice has
// been recorded yet (Chinese locales → zh, otherwise en).

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { Locale } from "./index";
import { translate, listLocales } from "./index";

interface I18nShape {
  locale: Locale;
  setLocale: (l: Locale) => void;
  t: (key: string) => string;
  locales: Locale[];
}

const I18nContext = createContext<I18nShape | null>(null);

const STORAGE_KEY = "dog.locale";

function detectInitial(): Locale {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "en" || stored === "zh") return stored;
  } catch (_) {
    // localStorage may be unavailable in some test runners; fall through.
  }
  const lang = (navigator?.language || "en").toLowerCase();
  return lang.startsWith("zh") ? "zh" : "en";
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(detectInitial);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    try { localStorage.setItem(STORAGE_KEY, l); } catch (_) {}
  }, []);

  const t = useCallback((key: string) => translate(locale, key), [locale]);

  const value = useMemo<I18nShape>(
    () => ({ locale, setLocale, t, locales: listLocales() as Locale[] }),
    [locale, setLocale, t]
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n outside I18nProvider");
  return ctx;
}
