import { fileURLToPath } from "node:url";
import tailwind from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// the Go service next door embeds dist/ and serves it, so nothing here is
// ever deployed on its own.
//
// the dev port and api target come from tools/dev.ts, which is the only thing
// that reads apps.toml; a bare `vite` just runs without a proxy.
const port = Number(process.env.RPI_WEB_PORT) || undefined;
const api = process.env.RPI_API_URL;

export default defineConfig({
    plugins: [react(), tailwind()],
    resolve: {
        alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) }
    },
    build: {
        outDir: "../server/dist",
        emptyOutDir: true,
        // the Go file server pins anything under this directory forever, the
        // same name the astro apps on the fleet use
        assetsDir: "_astro"
    },
    server: {
        ...(port ? { port } : {}),
        ...(api ? { proxy: { "/api": api } } : {})
    }
});
