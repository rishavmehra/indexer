import path from "path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  define: {
    "process.env.VITE_BACKEND_API_BASE_URL": JSON.stringify(
      process.env.VITE_BACKEND_API_BASE_URL
    ),
  },
});
