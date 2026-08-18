import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:18080",
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        // manualChunks splits vendor from app code. We do NOT carve
        // out per-route chunks here — that is handled by React.lazy
        // + dynamic imports inside App.tsx. The goal is to keep the
        // entry chunk under 100 KiB gz so the first paint lands in
        // a single round-trip.
        manualChunks: {
          react: ["react", "react-dom"],
          query: ["@tanstack/react-query"],
        },
      },
    },
    // Surface chunk sizes in the build output. The hard limit is
    // soft — we only log a warning so a brief regression does not
    // break CI.
    chunkSizeWarningLimit: 100,
  },
});
