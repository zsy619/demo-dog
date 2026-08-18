import { useCallback, useEffect, useState } from "react";

/**
 * useHashState persists a single value in the URL hash query string so it
 * survives Cmd+R reloads and produces shareable deep links.
 *
 * Two overloads:
 *   useHashState(key, initial: string)                                  // string
 *   useHashState<T>(key, initial: T, parse, stringify)                  // typed
 *
 * The hook only touches the query string portion of the current hash
 * (after the `?`) so it never collides with the page segment.
 *
 * It listens for `hashchange` so manual edits to the URL or external
 * navigations are picked up without remounting the page.
 */
export function useHashState(
  key: string,
  initial: string
): [string, (next: string) => void];
export function useHashState<T>(
  key: string,
  initial: T,
  parse: (raw: string) => T,
  stringify: (value: T) => string
): [T, (next: T) => void];
export function useHashState<T>(
  key: string,
  initial: T,
  parse?: (raw: string) => T,
  stringify?: (value: T) => string
): [T, (next: T) => void] {
  const p = parse ?? ((x: string) => x as unknown as T);
  const s = stringify ?? ((x: T) => String(x));
  return useHashStateImpl(key, initial, p, s);
}

function useHashStateImpl<T>(
  key: string,
  initial: T,
  parse: (raw: string) => T,
  stringify: (value: T) => string
): [T, (next: T) => void] {
  const read = useCallback((): T => {
    if (typeof window === "undefined") return initial;
    const h = window.location.hash;
    const qIdx = h.indexOf("?");
    if (qIdx < 0) return initial;
    const params = new URLSearchParams(h.slice(qIdx + 1));
    const raw = params.get(key);
    if (raw === null) return initial;
    try {
      return parse(raw);
    } catch {
      return initial;
    }
  }, [key, initial, parse]);

  const [value, setValue] = useState<T>(read);

  // Pick up manual hash changes (e.g. user pastes a new URL).
  useEffect(() => {
    const onHash = () => {
      const next = read();
      setValue((cur) => (cur === next ? cur : next));
    };
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, [read]);

  const write = useCallback(
    (next: T) => {
      setValue(next);
      if (typeof window === "undefined") return;
      const raw = stringify(next);
      const h = window.location.hash;
      const qIdx = h.indexOf("?");
      const base = qIdx >= 0 ? h.slice(0, qIdx) : h;
      const params = new URLSearchParams(
        qIdx >= 0 ? h.slice(qIdx + 1) : ""
      );
      if (!raw || raw === stringify(initial)) {
        params.delete(key);
      } else {
        params.set(key, raw);
      }
      const q = params.toString();
      const finalHash = q ? `${base}?${q}` : base;
      // Use replaceState for transient filter state so the back button stays
      // usable for page navigation rather than walking through every keystroke.
      try {
        window.history.replaceState(null, "", finalHash);
        // hashchange does NOT fire on replaceState, so nudge listeners.
        window.dispatchEvent(new HashChangeEvent("hashchange"));
      } catch {
        window.location.hash = finalHash;
      }
    },
    [key, initial, stringify]
  );

  return [value, write];
}

/**
 * useHashStateBool persists a boolean toggle as "1" / "" in the URL. Keeps
 * the URL short (no "true"/"false" string churn).
 */
export function useHashStateBool(
  key: string,
  initial: boolean
): [boolean, (next: boolean) => void] {
  const initStr: "1" | "" = initial ? "1" : "";
  const parse = (raw: string): boolean => raw === "1";
  const stringify = (val: boolean): string => (val ? "1" : "");
  const [v, setV] = useHashStateImpl<boolean>(
    key,
    initial,
    parse,
    stringify
  );
  // initialStr is only used so that the underlying impl removes the key when
  // the value matches the default; pass it through stringify itself.
  void initStr;
  return [v, setV];
}

/**
 * useHashStateJson persists an arbitrary JSON-serialisable value under a
 * single key. Uses base64-ish encoding via encodeURIComponent so the URL stays
 * readable. For small primitives prefer useHashState / useHashStateBool.
 */
export function useHashStateJson<T>(
  key: string,
  initial: T
): [T, (next: T) => void] {
  return useHashState(
    key,
    initial,
    (raw) => {
      try {
        return JSON.parse(decodeURIComponent(raw)) as T;
      } catch {
        return initial;
      }
    },
    (val) => encodeURIComponent(JSON.stringify(val))
  );
}
