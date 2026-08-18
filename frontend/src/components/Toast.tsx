import { useEffect, useState } from "react";

type ToastKind = "info" | "success" | "error";
interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
}

const listeners: Array<(t: Toast) => void> = [];
let nextId = 1;

export function toast(message: string, kind: ToastKind = "info") {
  const t: Toast = { id: nextId++, kind, message };
  for (const fn of listeners) fn(t);
}

export function ToastHost() {
  const [items, setItems] = useState<Toast[]>([]);

  useEffect(() => {
    const onToast = (t: Toast) => {
      setItems((prev) => [...prev, t]);
      window.setTimeout(() => {
        setItems((prev) => prev.filter((x) => x.id !== t.id));
      }, 3500);
    };
    listeners.push(onToast);
    return () => {
      const i = listeners.indexOf(onToast);
      if (i >= 0) listeners.splice(i, 1);
    };
  }, []);

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
      {items.map((t) => (
        <div
          key={t.id}
          className={`px-3 py-2 rounded text-sm shadow-lg border animate-fadeIn ${
            t.kind === "error"
              ? "bg-grafana-err/20 border-grafana-err/40 text-grafana-err"
              : t.kind === "success"
              ? "bg-grafana-ok/20 border-grafana-ok/40 text-grafana-ok"
              : "bg-grafana-elev2 border-grafana-border text-grafana-text"
          }`}
        >
          {t.message}
        </div>
      ))}
    </div>
  );
}
