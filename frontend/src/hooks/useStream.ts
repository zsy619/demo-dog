import { useEffect, useRef, useState } from "react";
import { getSharedStreamClient, type StreamMessage } from "@/lib/ws";

// useStream opens a shared WebSocket connection and returns the latest events.
// It auto-reconnects on disconnect via StreamClient.
//
// Usage:
//   const events = useStream({ max: 200, kinds: ["log", "metric"] });
//
// `events` is an array of StreamMessage, newest at the head.
export interface UseStreamOptions {
  max?: number;
  kinds?: string[];
  bufferMs?: number;
}

export interface UseStreamResult {
  events: StreamMessage[];
  status: "connecting" | "open" | "closed";
}

// useStream returns just the event ring buffer (legacy shape).
// Pages that need connection state should call useStreamStatus() instead.
export function useStream(opts: UseStreamOptions = {}): StreamMessage[] {
  const { max = 200, kinds, bufferMs = 0 } = opts;
  const [events, setEvents] = useState<StreamMessage[]>([]);
  const bufferRef = useRef<StreamMessage[]>([]);
  const flushTimer = useRef<number | null>(null);

  useEffect(() => {
    // Share a single WS connection across every page that uses the stream.
    const client = getSharedStreamClient();
    client.connect();

    const unsub = client.subscribe((msg) => {
      if (kinds && !kinds.includes(msg.kind)) return;
      bufferRef.current = [msg, ...bufferRef.current].slice(0, max);
      if (bufferMs <= 0) {
        setEvents(bufferRef.current);
      } else {
        if (flushTimer.current === null) {
          flushTimer.current = window.setTimeout(() => {
            flushTimer.current = null;
            setEvents([...bufferRef.current]);
          }, bufferMs);
        }
      }
    });

    return () => {
      unsub();
      if (flushTimer.current !== null) {
        window.clearTimeout(flushTimer.current);
        flushTimer.current = null;
      }
      // Do NOT close the singleton — other consumers may still be using it.
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [max, kinds?.join(","), bufferMs]);

  return events;
}

// useStreamStatus opens a transient WS connection purely to surface the
// current socket state for the TopBar status chip. It shares the same
// retry/backoff semantics as the regular stream client.
export function useStreamStatus(): "connecting" | "open" | "closed" {
  const [status, setStatus] = useState<"connecting" | "open" | "closed">(
    "connecting"
  );
  useEffect(() => {
    const client = getSharedStreamClient();
    client.connect();
    const unsub = client.onStatus(setStatus);
    return () => {
      unsub();
    };
  }, []);
  return status;
}
