// TanStack Query setup.
//
// We deliberately keep this thin: the demo has one QueryClient per
// app lifetime, the defaults are sensible for an internal SRE tool,
// and we expose typed `useXxxQuery` hooks per data domain so pages
// do not have to know about query keys or staleTime constants.

import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Re-fetch on focus so the operator sees fresh numbers when
      // they tab back from the WS dashboard. Network is local, the
      // cost is trivial.
      refetchOnWindowFocus: true,
      // 30s default. The pages that need shorter intervals (e.g.
      // TopBar health) re-fetch via dedicated hooks or a manual
      // invalidate().
      staleTime: 30_000,
      // We do not retry on 401 — the auth interceptor clears the
      // key and the UI surfaces a login modal. Retrying without
      // credentials is pointless and noisy.
      retry: (failureCount, err) => {
        const status = (err as { status?: number } | null)?.status;
        if (status === 401 || status === 403) return false;
        return failureCount < 2;
      },
    },
  },
});
