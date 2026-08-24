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
          // Keep the two authenticated application trees separate. Modules
          // shared by both trees get their own protected path so an admin
          // chunk can never accidentally import an asset guarded only by the
          // portal session middleware (and vice versa).
          if (chunkInfo.name === "PortalDashboardPage") {
            return "assets/portal/[name]-[hash].js";
          }
          if (chunkInfo.name === "AuthenticatedApp") {
            return "assets/secure/[name]-[hash].js";
          }
          return "assets/shared/[name]-[hash].js";
        },
      },
    },
  },
});
