// Thin wrapper around `useQuery` that injects the AbortController
// for cancellation on unmount. Without it, switching pages mid-
// fetch leaks an in-flight promise that lands on an unmounted
// component — React logs warnings and the page can flicker with
// stale data.
//
// Pages can still pass extra query options (refetchInterval,
// enabled, etc.) on top of what the wrapper provides.

import {
  useQuery,
  type UseQueryOptions,
  type QueryKey,
} from "@tanstack/react-query";
import { apiFetch, HttpError } from "@/lib/fetch";

type ApiQueryOptions<T> = Omit<
  UseQueryOptions<T, HttpError, T, QueryKey>,
  "queryKey" | "queryFn"
> & {
  // When set, the query is suppressed entirely. The fetch path is
  // skipped, no cancel/abort pair is created. Useful for cases
  // where a required argument (e.g. service name) is not yet known.
  skip?: boolean;
};

export function useApiQuery<T>(
  key: QueryKey,
  path: string,
  options: ApiQueryOptions<T> = {}
) {
  const { skip, ...rest } = options;
  return useQuery<T, HttpError, T, QueryKey>({
    queryKey: key,
    queryFn: async ({ signal }) => {
      return apiFetch<T>(path, { signal });
    },
    enabled: !skip && (options.enabled ?? true),
    ...rest,
  });
}
