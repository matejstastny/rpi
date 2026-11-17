// run one app's dev server and its Go api together, as `mise run dev <app>`.
import { existsSync } from "node:fs";
import { createServer } from "node:net";
import { createInterface } from "node:readline/promises";
import { stdin, stdout } from "node:process";
import { apiURL, app, apps, root, serverDir, webDir, webURL, type App } from "./manifest.ts";
import { start, run } from "./run.ts";
import { bold, cyan, die, green, muted, yellow } from "./term.ts";

function free(port: number): Promise<boolean> {
    return new Promise((resolve) => {
        const probe = createServer();
        probe.once("error", () => resolve(false));
        probe.once("listening", () => probe.close(() => resolve(true)));
        probe.listen(port, "127.0.0.1");
    });
}

async function preflight(a: App): Promise<void> {
    const taken = [];
    if (!(await free(a.api))) taken.push(`api ${a.api}`);
    if (!(await free(a.web))) taken.push(`frontend ${a.web}`);
    if (taken.length) {
        throw new Error(
            `${taken.join(" and ")} already in use, probably a dev server that did not exit.\n` +
                `       find it with: ss -lntp | grep -E '${a.api}|${a.web}'`
        );
    }
}

async function choose(): Promise<App> {
    const all = apps();
    if (!stdin.isTTY) {
        const first = all[0];
        if (!first) throw new Error("apps.toml has no apps");
        throw new Error(`name an app: mise run dev ${first.name}`);
    }

    console.log(`\n${bold("which app?")}`);
    all.forEach((a, i) =>
        console.log(`  ${cyan(String(i + 1))}  ${a.label} ${muted(`(${a.name})`)}`)
    );

    const input = createInterface({ input: stdin, output: stdout });
    const answer = (await input.question(`\n${green("›")} `)).trim();
    input.close();

    const picked = all[Number(answer) - 1];
    if (!picked) throw new Error("enter one of the listed numbers");
    return picked;
}

async function main() {
    const requested = process.argv[2];
    const a = requested ? app(requested) : await choose();
    const web = webDir(a);
    const server = serverDir(a);

    console.log(`\n${bold(a.label)} ${muted("local development")}`);
    console.log(`  frontend  ${cyan(webURL(a))}`);
    console.log(`  api       ${cyan(apiURL(a))}`);
    if (a.needs) console.log(`  ${yellow("requires")}  ${a.needs}`);
    console.log();

    await preflight(a);

    if (!existsSync(`${root}/node_modules`)) {
        console.log(muted("installing frontend dependencies…"));
        await run("pnpm", ["install", "--frozen-lockfile", "--silent"], { cwd: root });
    }

    // the Go binary embeds web/dist, so it has to exist before `go run` compiles
    console.log(muted("building the embedded frontend once…"));
    await run("pnpm", ["run", "build"], { cwd: web, quiet: true });

    console.log(`\n${green("ready")} ctrl-c stops both.\n`);

    const env = { RPI_WEB_PORT: String(a.web), RPI_API_URL: apiURL(a) };
    // each app's package.json owns how its frontend starts; astro backgrounds
    // itself when it detects a coding agent, which its own script opts out of
    const children = [
        start("pnpm", ["run", "dev"], { cwd: web, tag: "web", env }),
        start("go", ["run", "."], { cwd: server, tag: "api", env: a.env })
    ];

    let stopping = false;
    const stop = (signal: NodeJS.Signals) => {
        if (stopping) return;
        stopping = true;
        console.log(`\n${muted(`stopping ${a.name}…`)}`);
        for (const child of children) child.kill(signal);
    };
    process.once("SIGINT", () => stop("SIGINT"));
    process.once("SIGTERM", () => stop("SIGTERM"));

    await new Promise<void>((resolve) => {
        for (const child of children) {
            child.once("exit", (code, signal) => {
                if (!stopping) {
                    console.error(
                        `\n${yellow("warn:")} a process stopped (${signal ?? `status ${code}`})`
                    );
                    process.exitCode = code || 1;
                    stop("SIGTERM");
                }
                if (children.every((c) => c.exitCode !== null || c.signalCode !== null)) resolve();
            });
        }
    });
}

main().catch(die);
