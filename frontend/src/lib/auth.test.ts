import { describe, expect, it, beforeEach } from "vitest";
import {
  getApiKey,
  setApiKey,
  clearApiKey,
  getTenantId,
  setTenantId,
  isAuthed,
  authHeaders,
  subscribe,
} from "./auth";

describe("auth", () => {
  beforeEach(() => {
    localStorage.clear();
    clearApiKey();
    setTenantId("");
  });

  it("returns empty values initially", () => {
    expect(getApiKey()).toBe("");
    expect(getTenantId()).toBe("");
    expect(isAuthed()).toBe(false);
  });

  it("round-trips apiKey and persists to localStorage", () => {
    setApiKey("abc123");
    expect(getApiKey()).toBe("abc123");
    expect(isAuthed()).toBe(true);
    expect(localStorage.getItem("dog.apiKey")).toBe("abc123");
  });

  it("trims whitespace", () => {
    setApiKey("  xyz  ");
    expect(getApiKey()).toBe("xyz");
  });

  it("clearApiKey wipes state and storage", () => {
    setApiKey("k");
    clearApiKey();
    expect(getApiKey()).toBe("");
    expect(localStorage.getItem("dog.apiKey")).toBeNull();
  });

  it("authHeaders emits Authorization + X-Tenant-Id when set", () => {
    setApiKey("k1");
    setTenantId("acme");
    expect(authHeaders()).toEqual({
      Authorization: "Bearer k1",
      "X-Tenant-Id": "acme",
    });
  });

  it("authHeaders omits keys when empty", () => {
    expect(authHeaders()).toEqual({});
  });

  it("subscribe fires on every change", () => {
    const calls: number[] = [];
    const unsub = subscribe(() => calls.push(calls.length));
    setApiKey("a");
    setTenantId("b");
    clearApiKey();
    expect(calls.length).toBeGreaterThanOrEqual(3);
    unsub();
  });
});
