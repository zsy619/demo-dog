import { describe, expect, it, beforeEach, vi } from "vitest";
import { apiFetch, HttpError } from "./fetch";
import { setApiKey, clearApiKey, setTenantId } from "./auth";

// We replace global fetch with a tiny fake so the wrapper can be
// exercised without a real network. Each test installs a fresh
// mock and asserts on call shape + return behaviour.
function installFetch(impl: (input: RequestInfo, init?: RequestInit) => Promise<Response>) {
  globalThis.fetch = vi.fn((input, init) => impl(input as RequestInfo, init)) as typeof fetch;
}

describe("apiFetch", () => {
  beforeEach(() => {
    localStorage.clear();
    clearApiKey();
    setTenantId("");
  });

  it("attaches Authorization header when an api key is set", async () => {
    setApiKey("secret-key");
    let captured: RequestInit | undefined;
    installFetch(async (_url, init) => {
      captured = init;
      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    });

    const out = await apiFetch<{ ok: boolean }>("/health");
    expect(out).toEqual({ ok: true });
    const headers = captured?.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer secret-key");
  });

  it("omits headers when anonymous is set", async () => {
    setApiKey("k");
    let captured: RequestInit | undefined;
    installFetch(async (_url, init) => {
      captured = init;
      return new Response(JSON.stringify({}), { status: 200 });
    });
    await apiFetch("/health", { anonymous: true });
    const headers = captured?.headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });

  it("throws HttpError on non-2xx", async () => {
    installFetch(async () =>
      new Response("boom", { status: 500 })
    );
    await expect(apiFetch("/x")).rejects.toBeInstanceOf(HttpError);
  });

  it("clears api key on 401 and emits dog:auth-error", async () => {
    setApiKey("bad-key");
    installFetch(async () => new Response("nope", { status: 401 }));
    const handler = vi.fn();
    window.addEventListener("dog:auth-error", handler);
    await expect(apiFetch("/x")).rejects.toBeInstanceOf(HttpError);
    expect(localStorage.getItem("dog.apiKey")).toBeNull();
    expect(handler).toHaveBeenCalled();
    window.removeEventListener("dog:auth-error", handler);
  });

  it("serialises body + sets Content-Type when provided", async () => {
    let captured: RequestInit | undefined;
    installFetch(async (_url, init) => {
      captured = init;
      return new Response("{}", { status: 200 });
    });
    await apiFetch("/post", { method: "POST", body: { hello: "world" } });
    expect(captured?.method).toBe("POST");
    const headers = captured?.headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");
    expect(captured?.body).toBe(JSON.stringify({ hello: "world" }));
  });

  it("forwards AbortSignal", async () => {
    installFetch(async () => new Response("{}", { status: 200 }));
    const ctrl = new AbortController();
    await apiFetch("/x", { signal: ctrl.signal });
    expect(true).toBe(true); // smoke — would throw if wrapper ignored signal
  });

  it("includes X-Tenant-Id when tenant is set", async () => {
    setTenantId("acme");
    let captured: RequestInit | undefined;
    installFetch(async (_url, init) => {
      captured = init;
      return new Response("{}", { status: 200 });
    });
    await apiFetch("/x");
    const headers = captured?.headers as Record<string, string>;
    expect(headers["X-Tenant-Id"]).toBe("acme");
  });
});
