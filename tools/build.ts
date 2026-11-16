// cross-compile one or more apps for the pis, as `mise run build [app...]`.
// each binary embeds its own frontend build, so a host needs neither node nor
// a static root. pi reads build/ when syncing with --services.
import { mkdirSync, rmSync, copyFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { apps, root, serverDir, webDir, type App } from "./manifest.ts";
import { run } from "./run.ts";
import { bytes, need } from "./shell.ts";
import { die, done, muted, step } from "./term.ts";

const buildDir = join(root, "build");

// the pis are all 64-bit raspberry pis running alpine
const target = { GOOS: "linux", GOARCH: "arm64", CGO_ENABLED: "0" };

async function build(a: App) {
    step(`${a.name}: ui`);
    await run("pnpm", ["run", "build"], { cwd: webDir(a), quiet: true });

    step(`${a.name}: binary for ${target.GOOS}/${target.GOARCH}`);
    const out = join(buildDir, a.name);
    await run("go", ["build", "-trimpath", "-ldflags=-s -w", "-o", out, "."], {
        cwd: serverDir(a),
        env: target
    });
    console.log(muted(`  ${bytes(statSync(out).size)} at build/${a.name}`));

    for (const f of a.files ?? []) {
        copyFileSync(join(root, a.name, f.from), join(buildDir, f.as));
        console.log(muted(`  build/${f.as}`));
    }
}

async function main() {
    need("pnpm");
    need("go");

    const names = process.argv.slice(2);
    const selected = names.length ? apps().filter((a) => names.includes(a.name)) : apps();
    const unknown = names.filter((n) => !selected.some((a) => a.name === n));
    if (unknown.length) {
        throw new Error(
            `unknown app: ${unknown.join(", ")}, have: ${apps()
                .map((a) => a.name)
                .join(", ")}`
        );
    }

    // a stale binary in build/ would be shipped as if it were current
    if (!names.length) rmSync(buildDir, { recursive: true, force: true });
    mkdirSync(buildDir, { recursive: true });

    await run("pnpm", ["install", "--frozen-lockfile", "--silent"], { cwd: root });
    for (const a of selected) await build(a);

    done(`built ${selected.map((a) => a.name).join(", ")} into build/`);
}

main().catch(die);
