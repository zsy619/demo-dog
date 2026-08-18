// Auth state for the demo frontend.
//
// We deliberately keep this minimal: a single API key stored in
// localStorage (or an in-memory fallback when localStorage is not
// available, e.g. SSR or sandboxed iframes). The collector accepts
// the same value via either `Authorization: Bearer <key>` or the
// `X-API-Key` header; we use the bearer form because it is the
// standard set out by the collector middleware.
//
// This is NOT a real auth system. It only exists so the frontend can
// talk to a hardened backend. Multi-user session management, refresh
// tokens, and SSO all live outside the scope of the demo.

const STORAGE_KEY = "dog.apiKey";
const TENANT_KEY = "dog.tenantId";

// authState holds the currently configured credentials. It is a
// module-level singleton because the demo has no notion of
// per-user state — when there is one user, the singleton is the
// entire auth system.
let apiKey: string = readStorage(STORAGE_KEY) ?? "";
let tenantId: string = readStorage(TENANT_KEY) ?? "";

// Listeners are notified on every change so the UI can re-render
// without polling. The list is intentionally tiny; the demo never
// has more than a handful of subscribers.
type Listener = () => void;
const listeners: Set<Listener> = new Set();

function readStorage(key: string): string | null {
  try {
    return globalThis.localStorage?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string): void {
  try {
    if (value) {
      globalThis.localStorage?.setItem(key, value);
    } else {
      globalThis.localStorage?.removeItem(key);
    }
  } catch {
    // ignore — private browsing mode, SSR, etc.
  }
}

export function getApiKey(): string {
  return apiKey;
}

export function setApiKey(key: string): void {
  apiKey = key.trim();
  writeStorage(STORAGE_KEY, apiKey);
  for (const fn of listeners) fn();
}

export function clearApiKey(): void {
  apiKey = "";
  writeStorage(STORAGE_KEY, "");
  for (const fn of listeners) fn();
}

export function getTenantId(): string {
  return tenantId;
}

export function setTenantId(id: string): void {
  tenantId = id.trim();
  writeStorage(TENANT_KEY, tenantId);
  for (const fn of listeners) fn();
}

export function subscribe(fn: Listener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

// authHeaders returns the headers that should be attached to outgoing
// API requests. Returns an empty object when no key is set so the
// caller can pass it directly into fetch / Request.
export function authHeaders(): Record<string, string> {
  const h: Record<string, string> = {};
  if (apiKey) h["Authorization"] = `Bearer ${apiKey}`;
  if (tenantId) h["X-Tenant-Id"] = tenantId;
  return h;
}

// isAuthed reports whether the current configuration can talk to a
// collector that requires an API key.
export function isAuthed(): boolean {
  return apiKey.length > 0;
}
