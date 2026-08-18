/** @type {import("tailwindcss").Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // Grafana-like dark palette
        grafana: {
          bg: "#0b0d12",
          panel: "#111317",
          elev: "#181c22",
          border: "#22272e",
          text: "#d8d9da",
          muted: "#9fa6b2",
          accent: "#3b82f6",
          accent2: "#7e3bf2",
          ok: "#1a7f37",
          warn: "#bf8700",
          err: "#cf222e",
        },
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "SFMono-Regular", "monospace"],
      },
    },
  },
  plugins: [],
};
