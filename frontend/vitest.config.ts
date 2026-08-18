import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    css: false,
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      thresholds: {
        // Round 24 sets a 70% line coverage floor on the well-tested
        // core: useApiQuery wrapper, fetch/auth lib helpers, the
        // queries hooks surface, and the two big UI primitives
        // (LoginModal, VirtualTable). Pages and the larger lib
        // surface (router/ws/time) are exercised by Playwright
        // E2E and live outside this threshold.
        lines: 70,
        functions: 70,
        statements: 70,
        branches: 50,
      },
      include: [
        "src/hooks/useApiQuery.ts",
        "src/hooks/queries.ts",
        "src/lib/auth.ts",
        "src/lib/fetch.ts",
        "src/lib/query.ts",
        "src/components/LoginModal.tsx",
        "src/components/VirtualTable.tsx",
      ],
      exclude: [
        "src/**/*.test.{ts,tsx}",
        "src/test-setup.ts",
        "src/main.tsx",
        "src/pages/**",
        "src/components/anim/**",
        "src/hooks/useAuth.ts",
        "src/hooks/useHashState.ts",
        "src/hooks/useStream.ts",
      ],
    },
  },
});
