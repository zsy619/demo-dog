// Thin wrapper around fetch that automatically attaches auth headers
// (Authorization: Bearer + X-Tenant-Id) and parses JSON responses.
//
// All api.ts helpers route through here so callers never need to
// remember to inject the token. The wrapper also normalises 401
// responses into a structured error so the UI can prompt the user
// to log in.

import { authHeaders, clearApiKey } from "./auth";

export class HttpError extends Error {
  status: number;
  body: string;
  constructor(status: number, body: string) {
    super(`HTTP ${status}: ${body.slice(0, 200)}`);
    this.name = "HttpError";
    this.status = status;
    this.body = body;
  }
}

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
  // Custom headers to attach (merged with default Content-Type/Accept).
  // Useful for endpoints that require a non-JSON Content-Type
  // (e.g. Prometheus remote write uses application/x-protobuf).
  headers?: Record<string, string>;
  // When true the request omits the Authorization header. Used by
  // the login modal itself, which obviously cannot present a key it
  // does not have yet.
  anonymous?: boolean;
}

export async function apiFetch<T>(
  path: string,
  opts: RequestOptions = {}
): Promise<T> {
  const headers: Record<string, string> = {
    Accept: "application/json",
  };
  if (opts.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (!opts.anonymous) {
    Object.assign(headers, authHeaders());
  }
  // Allow callers to override or add headers (e.g. for non-JSON
  // endpoints like Prometheus remote write).
  if (opts.headers) {
    Object.assign(headers, opts.headers);
  }

  const init: RequestInit = {
    method: opts.method ?? "GET",
    headers,
  };
  if (opts.body !== undefined) init.body = JSON.stringify(opts.body);
  if (opts.signal) init.signal = opts.signal;

  const res = await fetch(`/api${path}`, init);
  const text = await res.text();
  if (!res.ok) {
    // 401 from the collector means the key was rejected. Clear it
    // so the user is prompted to log in again rather than silently
    // retrying with a stale credential.
    if (res.status === 401) {
      clearApiKey();
      // Bubble a window event so the App can pop the login modal
      // without coupling the fetch layer to React.
      if (typeof window !== "undefined") {
        window.dispatchEvent(
          new CustomEvent("dog:auth-error", { detail: { status: 401 } })
        );
      }
    }
    throw new HttpError(res.status, text);
  }
  // Some endpoints return empty body (e.g. 204); be permissive.
  if (!text) return undefined as T;
  try {
    return JSON.parse(text) as T;
  } catch (err) {
    throw new HttpError(res.status, `invalid JSON: ${String(err)}`);
  }
}
