import { spawn, type ChildProcess } from "node:child_process";
import { existsSync } from "node:fs";
import { createInterface } from "node:readline/promises";
import { stdin, stdout } from "node:process";

type Service = {
    name: string;
    label: string;
    web: string;
    server: string;
    webURL: string;
    apiURL: string;
    apiEnv?: Record<string, string>;
    needs?: string;
};

const services: Service[] = [
    {
        name: "ytdl",
        label: "YouTube downloader",
        web: "services/io/web",
        server: "services/io/server",
        webURL: "http://localhost:4321",
        apiURL: "http://127.0.0.1:8090",
        apiEnv: {
            YTDL_CACHE_DIR: "/tmp/rpi-ytdl/cache",
            YTDL_VIDEO_DIR: "/tmp/rpi-ytdl/video",
            YTDL_AUDIO_DIR: "/tmp/rpi-ytdl/audio"
        },
        needs: "yt-dlp and ffmpeg to download media"
    },
    {
        name: "share",
        label: "Quick share",
        web: "services/io/share-web",
        server: "services/io/share-server",
        webURL: "http://localhost:4322",
        apiURL: "http://127.0.0.1:8091",
        apiEnv: { SHARE_DIR: "/tmp/rpi-share" }
    },
    {
        name: "dash",
        label: "Fleet dashboard",
        web: "services/io/dash-web",
        server: "services/io/dash-server",
        webURL: "http://localhost:4323",
        apiURL: "http://127.0.0.1:8092",
        apiEnv: { DASH_CATALOG: "./catalog.json" },
        needs: "VictoriaMetrics, Transmission, Jellyfin, and Tailscale for live data"
    },
    {
        name: "torrent",
        label: "Transmission UI",
        web: "services/io/torrent-web",
        server: "services/io/torrent-server",
        webURL: "http://localhost:4324",
        apiURL: "http://127.0.0.1:8093",
        needs: "Transmission RPC for live data"
    },
    {
        name: "watch",
        label: "Watchlist",
        web: "services/io/watch-web",
        server: "services/io/watch-server",
        webURL: "http://localhost:4324",
        apiURL: "http://127.0.0.1:8095",
        apiEnv: {
            WATCH_DIR: "/tmp/rpi-watch",
            WATCH_SEED: "../watch-seed.json"
        }
    }
];

const colour = stdout.isTTY;
const paint = (code: number, value: string) =>
    colour ? `\u001B[${code}m${value}\u001B[0m` : value;
const heading = (value: string) => paint(1, value);
const muted = (value: string) => paint(90, value);
const green = (value: string) => paint(32, value);
const cyan = (value: string) => paint(36, value);
const yellow = (value: string) => paint(33, value);

function run(
    command: string,
    args: string[],
    cwd: string,
    prefix: string,
    env?: Record<string, string>
): ChildProcess {
    const child = spawn(command, args, {
        cwd,
        env: { ...process.env, ...env },
        stdio: ["inherit", "pipe", "pipe"]
    });
    const write = (chunk: Buffer, destination: NodeJS.WriteStream) => {
        const tag = muted(`${prefix.padEnd(8)} │ `);
        const text = chunk.toString();
        destination.write(
            text
                .split(/(?<=\n)/)
                .map((line) => `${tag}${line}`)
                .join("")
        );
    };

    child.stdout?.on("data", (chunk: Buffer) => write(chunk, stdout));
    child.stderr?.on("data", (chunk: Buffer) => write(chunk, process.stderr));
    child.on("error", (error) => console.error(`${prefix}: ${error.message}`));
    return child;
}

async function chooseService(): Promise<Service> {
    if (!stdin.isTTY) {
        throw new Error(`Choose a service explicitly, for example: pnpm ${services[0].name}:dev`);
    }

    console.log(`\n${heading("Choose a local service")}`);
    services.forEach((service, index) => {
        console.log(`  ${cyan(String(index + 1))}  ${service.label} ${muted(`(${service.name})`)}`);
    });

    const input = createInterface({ input: stdin, output: stdout });
    const answer = (await input.question(`\n${green("›")} `)).trim();
    input.close();
    const choice = Number(answer);
    const service = services[choice - 1];
    if (!Number.isInteger(choice) || !service) {
        throw new Error("Enter one of the listed numbers.");
    }
    return service;
}

async function waitFor(child: ChildProcess): Promise<void> {
    await new Promise<void>((resolve, reject) => {
        child.once("error", reject);
        child.once("exit", (code) => {
            if (code === 0) resolve();
            else reject(new Error(`exited with status ${code ?? "signal"}`));
        });
    });
}

async function prepare(service: Service): Promise<void> {
    if (!existsSync(`${service.web}/node_modules`)) {
        console.log(muted("Installing frontend dependencies..."));
        await waitFor(
            run("pnpm", ["install", "--frozen-lockfile", "--silent"], service.web, "install")
        );
    }

    console.log(muted("Building the embedded frontend once for the Go server..."));
    await waitFor(run("pnpm", ["build"], service.web, "build"));
}

async function main(): Promise<void> {
    const requested = process.argv[2];
    const service = requested
        ? services.find((item) => item.name === requested)
        : await chooseService();
    if (!service) {
        throw new Error(
            `Unknown service ${JSON.stringify(requested)}. Choose one of: ${services.map((item) => item.name).join(", ")}`
        );
    }

    console.log(`\n${heading(service.label)} ${muted("local development")}`);
    console.log(`  frontend  ${cyan(service.webURL)}`);
    console.log(`  API       ${cyan(service.apiURL)}`);
    if (service.needs) console.log(`  ${yellow("requires")}  ${service.needs}`);
    console.log();

    await prepare(service);

    console.log(`\n${green("ready")} Starting frontend and API. Press Ctrl-C to stop both.\n`);
    // astro detects coding-agent environments and backgrounds `astro dev`, so the launcher opts out to own the process
    const web = run("pnpm", ["exec", "astro", "dev", "--ignore-lock"], service.web, "web");
    const api = run("go", ["run", "."], service.server, "api", service.apiEnv);
    const children = [web, api];
    let stopping = false;

    const stop = (signal: NodeJS.Signals) => {
        if (stopping) return;
        stopping = true;
        console.log(`\n${muted(`Stopping ${service.name}…`)}`);
        for (const child of children) child.kill(signal);
    };

    process.once("SIGINT", () => stop("SIGINT"));
    process.once("SIGTERM", () => stop("SIGTERM"));

    await new Promise<void>((resolve) => {
        for (const child of children) {
            child.once("exit", (code, signal) => {
                if (!stopping) {
                    console.error(
                        `${child === web ? "web" : "api"} stopped (${signal ?? `status ${code}`})`
                    );
                    stop("SIGTERM");
                    process.exitCode = code || 1;
                }
                if (children.every((item) => item.exitCode !== null || item.signalCode !== null))
                    resolve();
            });
        }
    });
}

main().catch((error: Error) => {
    console.error(`\n${yellow("error:")} ${error.message}`);
    process.exitCode = 1;
});
