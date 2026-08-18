import "@testing-library/jest-dom/vitest";

// jsdom does not implement matchMedia / ResizeObserver. The
// LoginModal + TopBar do not depend on either, but virtualised
// components sometimes ask for a non-zero viewport. Stub both.
class RO {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as unknown as { ResizeObserver: typeof RO }).ResizeObserver = RO;

if (!("matchMedia" in window)) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (q: string) => ({
      matches: false,
      media: q,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

// jsdom 25 + vitest 2 occasionally reports localStorage as
// undefined when the global is not explicitly primed. Mirror a
// tiny in-memory storage on globalThis so the auth lib + tests
// can read/write/clear without crashing.
if (typeof globalThis.localStorage === "undefined") {
  const store = new Map<string, string>();
  const fakeStorage = {
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    setItem: (k: string, v: string) => {
      store.set(k, String(v));
    },
    removeItem: (k: string) => {
      store.delete(k);
    },
    clear: () => {
      store.clear();
    },
    key: (i: number) => Array.from(store.keys())[i] ?? null,
    get length() {
      return store.size;
    },
  };
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: fakeStorage,
  });
}
