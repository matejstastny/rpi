import { readFileSync, readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "smol-toml";

export const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

export type AppFile = { from: string; as: string };

export type App = {
    name: string;
    label: string;
    host: string;
    domain: string;
    /** loopback port the Go api binds, in dev and on the host alike */
    api: number;
    /** port the astro dev server uses locally */
    web: number;
    needs?: string;
    env?: Record<string, string>;
    files?: AppFile[];
};

export type Host = {
    name: string;
    glyph: string;
    plane: string;
    blurb: string;
    colors: { accent: string; bright: string; alt: string; dim: string };
    services: string[];
};

function read<T>(path: string): T {
    return parse(readFileSync(path, "utf8")) as T;
}

/** apps returns every app in apps.toml, in declaration order. */
export function apps(): App[] {
    const { app } = read<{ app: App[] }>(join(root, "apps.toml"));
    const seen = new Map<number, string>();
    for (const a of app) {
        for (const [port, kind] of [
            [a.api, "api"],
            [a.web, "web"]
        ] as const) {
            const clash = seen.get(port);
            if (clash)
                throw new Error(`apps.toml: ${a.name} ${kind} port ${port} collides with ${clash}`);
            seen.set(port, `${a.name} ${kind}`);
        }
    }
    return app;
}

/** app looks one app up by name, listing the alternatives when it misses. */
export function app(name: string): App {
    const all = apps();
    const found = all.find((a) => a.name === name);
    if (!found) {
        throw new Error(
            `unknown app ${JSON.stringify(name)}, have: ${all.map((a) => a.name).join(", ")}`
        );
    }
    return found;
}

/** hosts returns every pi that has a hosts/<name>/host.toml. */
export function hosts(): Host[] {
    return readdirSync(join(root, "hosts"), { withFileTypes: true })
        .filter((e) => e.isDirectory())
        .map((e) => read<Host>(join(root, "hosts", e.name, "host.toml")))
        .sort((a, b) => a.name.localeCompare(b.name));
}

export const webDir = (a: App) => join(root, a.name, "web");
export const serverDir = (a: App) => join(root, a.name, "server");
export const apiURL = (a: App) => `http://127.0.0.1:${a.api}`;
export const webURL = (a: App) => `http://localhost:${a.web}`;
