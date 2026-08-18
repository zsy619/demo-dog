// Thin WebSocket wrapper that re-opens on disconnect and surfaces a
// subscription callback. The backend exposes its event stream at /api/stream.

import type { StreamEvent } from "@/types/api";
import { getApiKey, getTenantId } from "./auth";

// StreamMessage mirrors the server StreamEvent shape; we keep both so
// pages that want the strict union (StreamEvent) can also use it.
export type StreamMessage = StreamEvent;

type Listener = (msg: StreamMessage) => void;
export type StreamStatus = "connecting" | "open" | "closed";
type StatusListener = (s: StreamStatus) => void;

export class StreamClient {
  private socket: WebSocket | null = null;
  private listeners = new Set<Listener>();
  private statusListeners = new Set<StatusListener>();
  private url: string;
  private retries = 0;
  private closed = false;
  private reconnectTimer: number | null = null;
  private _status: StreamStatus = "closed";

  get status(): StreamStatus {
    return this._status;
  }

  private setStatus(s: StreamStatus) {
    this._status = s;
    this.statusListeners.forEach((l) => l(s));
  }

  onStatus(fn: StatusListener): () => void {
    this.statusListeners.add(fn);
    fn(this._status);
    return () => this.statusListeners.delete(fn);
  }

  constructor(url?: string) {
    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    // Browsers cannot set arbitrary headers on WebSocket. We pass
    // the API key (and optional tenant id) as query parameters so
    // the collector middleware can authenticate the handshake. The
    // query-param path is explicitly supported on the backend via
    // AuthModeAPIKey.extractKey(); do not enable in production
    // unless you accept tokens landing in access logs.
    const params = new URLSearchParams();
    const k = getApiKey();
    if (k) params.set("api_key", k);
    const t = getTenantId();
    if (t) params.set("tenant", t);
    const q = params.toString();
    this.url =
      url ??
      `${proto}://${window.location.host}/api/stream${q ? `?${q}` : ""}`;
  }

  connect() {
    if (this.closed) return;
    this.closed = false;
    this.open();
  }

  private open() {
    try {
      this.setStatus("connecting");
      this.socket = new WebSocket(this.url);
    } catch (err) {
      console.warn("[ws] failed to construct", err);
      this.setStatus("closed");
      this.scheduleReconnect();
      return;
    }
    this.socket.onopen = () => this.setStatus("open");
    this.socket.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as StreamMessage;
        this.listeners.forEach((l) => l(data));
      } catch (e) {
        console.warn("[ws] bad frame", e);
      }
    };
    this.socket.onclose = () => {
      this.socket = null;
      this.setStatus("closed");
      this.scheduleReconnect();
    };
    this.socket.onerror = () => {
      try {
        this.socket?.close();
      } catch (e) {
        void e;
      }
    };
    this.retries = 0;
  }

  private scheduleReconnect() {
    if (this.closed) return;
    this.retries++;
    const delay = Math.min(1000 * 2 ** Math.min(this.retries, 5), 8000);
    if (this.reconnectTimer !== null) return;
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.open();
    }, delay);
  }

  subscribe(fn: Listener): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  close() {
    this.closed = true;
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.listeners.clear();
    this.socket?.close();
    this.socket = null;
  }
}

// Module-level singleton so every page that calls useStream() /
// useStreamStatus() shares a single WebSocket connection. The previous
// design created one socket per hook instance, which (1) wasted backend
// connections and (2) caused N reconnect storms when the backend went
// down.
let _shared: StreamClient | null = null;

export function getSharedStreamClient(): StreamClient {
  if (_shared === null) {
    _shared = new StreamClient();
  }
  return _shared;
}
