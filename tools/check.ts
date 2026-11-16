// the whole repository's checks, as `mise run check`. one pass per language so
// a failure says which half of the repo broke.
import { execFileSync } from "node:child_process";
import { apps, root, webDir } from "./manifest.ts";
import { run } from "./run.ts";
import { die, done, muted, step } from "./term.ts";

const git = (...args: string[]) =>
    execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim().split("\n").filter(Boolean);

const fix = process.argv.includes("--fix");

async function go() {
    step("go: format");
    const files = git("ls-files", "*.go");
    if (files.length) {
        const diff = execFileSync("gofmt", [fix ? "-w" : "-l", ...files], {
            cwd: root,
            encoding: "utf8"
        }).trim();
        if (diff && !fix) throw new Error(`not gofmt-formatted:\n${diff}\nrun: mise run fmt`);
    }

    step("go: lint");
    await run("golangci-lint", ["run", "./..."], { cwd: root });

    step("go: vet");
    await run("go", ["vet", "./..."], { cwd: root });

    step("go: test");
    await run("go", ["test", "./..."], { cwd: root });
}

async function webPasses() {
    step("web: install");
    await run("pnpm", ["install", "--frozen-lockfile", "--silent"], { cwd: root });

    step("web: format");
    await run("pnpm", ["exec", "prettier", fix ? "--write" : "--check", "."], {
        cwd: root,
        quiet: !fix
    });

    for (const a of apps()) {
        step(`web: ${a.name} typecheck and build`);
        await run("pnpm", ["run", "typecheck"], { cwd: webDir(a), quiet: true });
        await run("pnpm", ["run", "build"], { cwd: webDir(a), quiet: true });
    }
}

async function shell() {
    step("shell: syntax");
    const scripts = git("ls-files", "bin/*", "hosts/*/install", "*.initd", "*.confd");
    for (const f of scripts) await run("sh", ["-n", f], { cwd: root });
    console.log(muted(`  ${scripts.length} scripts parse`));
}

async function python() {
    step("python: format and lint");
    await run("ruff", ["format", fix ? "" : "--check", "."].filter(Boolean), {
        cwd: root,
        quiet: !fix
    });
    await run("ruff", ["check", ...(fix ? ["--fix"] : []), "."], { cwd: root, quiet: !fix });

    step("python: test");
    await run("python3", ["-m", "unittest", "discover", "-s", "cli", "-t", ".", "-q"], {
        cwd: root,
        quiet: true
    });
}

const stages = { go, web: webPasses, shell, python };

async function main() {
    const only = process.argv.slice(2).filter((a) => !a.startsWith("--"));
    const names = only.length ? only : Object.keys(stages);

    for (const name of names) {
        const stage = stages[name as keyof typeof stages];
        if (!stage)
            throw new Error(`unknown stage ${name}, have: ${Object.keys(stages).join(", ")}`);
        await stage();
    }

    done(fix ? "formatted and checked" : "all checks passed");
}

main().catch(die);
