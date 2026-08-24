import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: false,
    proxy: {
      "/api": "http://127.0.0.1:8256",
      "/health": "http://127.0.0.1:8256",
    },
  },
  build: {
    outDir: "dist",
    sourcemap: false,
    rollupOptions: {
      output: {
        entryFileNames: "assets/[name]-[hash].js",
        chunkFileNames: (chunkInfo) => {
          // Keep the two authenticated application trees separate. The
          // portal chunk is requested only after login and is served through
          // the portal session middleware; it must never be a public asset.
          const portalChunks = new Set(["PortalDashboardPage", "user-round"]);
          return portalChunks.has(chunkInfo.name)
            ? "assets/portal/[name]-[hash].js"
            : "assets/secure/[name]-[hash].js";
        },
      },
    },
  },
});
