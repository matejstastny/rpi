// @ts-check
import { defineConfig } from "astro/config";

// the Go service next door embeds dist/ and serves it, so nothing here is ever
// deployed on its own. directory format keeps sub-pages working under
// http.FileServer.
//
// the dev port and api target come from tools/dev.ts, which is the only thing
// that reads apps.toml; a bare `astro dev` just runs without a proxy.
const port = Number(process.env.RPI_WEB_PORT) || undefined;
const api = process.env.RPI_API_URL;

export default defineConfig({
    // pages/, layouts/, scripts/ and styles/ sit straight in this directory;
    // there are only four of them and an extra src/ to open every time is noise
    srcDir: ".",
    outDir: "../server/dist",
    build: { format: "directory", assets: "_astro" },
    devToolbar: { enabled: false },
    ...(port ? { server: { port } } : {}),
    ...(api ? { vite: { server: { proxy: { "/api": api, "/dl": api } } } } : {})
});
