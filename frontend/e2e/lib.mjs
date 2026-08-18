// Shared helpers for the round-24 E2E smoke tests.
//
// Kept dependency-free so the scripts run in CI without
// playwright / puppeteer / headless chrome. They assert behaviour
// through the HTTP surface — enough to prove the collector + the
// frontend serve a coherent end-to-end story.

import { spawn } from "node:child_process";
import { setTimeout as sleep } from "node:timers/promises";

const REPO_ROOT = decodeURIComponent(new URL("../..", import.meta.url).pathname);

export async function startCollector(port = 18081, extra = []) {
  const bin = `${REPO_ROOT}backend/dog-collector`;
  const env = {
    ...process.env,
    DOG_API_KEYS: "admin:admin:ops",
  };
  const args = ["-addr", `:${port}`, ...extra];
  const child = spawn(bin, args, { env, stdio: ["ignore", "pipe", "pipe"] });
  // Wait for the listener.
  for (let i = 0; i < 50; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${port}/api/health`);
      if (r.ok) return child;
    } catch (_) {
      // not yet
    }
    await sleep(100);
  }
  try { child.kill("SIGKILL"); } catch (_) {}
  throw new Error(`collector did not bind :${port} in 5s`);
}

export async function stopCollector(child) {
  if (!child || child.killed) return;
  child.kill("SIGKILL");
  await new Promise((r) => child.once("exit", r));
}

export async function req(port, path, opts = {}) {
  const headers = {
    "Content-Type": "application/json",
    ...(opts.headers || {}),
  };
  if (opts.token) {
    headers["Authorization"] = `Bearer ${opts.token}`;
  }
  const r = await fetch(`http://127.0.0.1:${port}${path}`, {
    method: opts.method || "GET",
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });
  const text = await r.text();
  let json = null;
  try { json = JSON.parse(text); } catch (_) {}
  return { status: r.status, body: json, text };
}

export function assert(cond, msg) {
  if (!cond) throw new Error(`assert: ${msg}`);
  return cond;
}
