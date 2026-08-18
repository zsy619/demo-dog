// Tiny URL-hash router for the SPA.
//
// Format: #/page?key=value&key2=value2
// Example: #/explore?signal=logs&service=checkout
//
// Hooks return the current params and a setter that updates the URL.
// The router does not own state; it just mirrors it to/from the URL.

import { useEffect, useState, useCallback } from "react";

export interface RouteState {
  page: string;
  params: URLSearchParams;
}

function parseHash(): RouteState {
  const h = window.location.hash || "#/overview";
  const cleaned = h.startsWith("#") ? h.slice(1) : h;
  const [path, search] = cleaned.split("?");
  const page = (path || "/overview").replace(/^\//, "");
  return {
    page,
    params: new URLSearchParams(search || ""),
  };
}

export function useRoute(): [string, URLSearchParams, (page: string, params?: URLSearchParams | Record<string, string>) => void] {
  const [state, setState] = useState<RouteState>(parseHash);

  useEffect(() => {
    const onHash = () => setState(parseHash());
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  const navigate = useCallback(
    (page: string, params?: URLSearchParams | Record<string, string>) => {
      let qs = "";
      if (params instanceof URLSearchParams) {
        qs = params.toString();
      } else if (params) {
        const sp = new URLSearchParams();
        for (const [k, v] of Object.entries(params)) {
          if (v !== undefined && v !== null && v !== "") {
            sp.set(k, String(v));
          }
        }
        qs = sp.toString();
      }
      const target = `#/${page}${qs ? "?" + qs : ""}`;
      if (window.location.hash !== target) {
        window.location.hash = target;
      } else {
        // Force a re-render even if hash didn't change.
        setState(parseHash());
      }
    },
    []
  );

  return [state.page, state.params, navigate];
}

// buildHash constructs a #/page?... string from a page name and an object.
export function buildHash(page: string, params?: Record<string, string>): string {
  const sp = new URLSearchParams();
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== "") sp.set(k, v);
    }
  }
  return `#/${page}${sp.toString() ? "?" + sp.toString() : ""}`;
}
