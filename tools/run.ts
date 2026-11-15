import { spawn, type ChildProcess } from "node:child_process";
import { muted } from "./term.ts";

export type Options = {
    cwd: string;
    /** prefixed in front of every output line, so two processes stay legible */
    tag?: string;
    env?: Record<string, string>;
    quiet?: boolean;
};

/** spawn starts a child, tagging its output when a tag is given. */
export function start(command: string, args: string[], opts: Options): ChildProcess {
    const child = spawn(command, args, {
        cwd: opts.cwd,
        env: { ...process.env, ...opts.env },
        stdio: [
            "inherit",
            opts.tag || opts.quiet ? "pipe" : "inherit",
            opts.tag || opts.quiet ? "pipe" : "inherit"
        ]
    });

    if (!opts.tag) {
        if (opts.quiet) {
            // keep stderr: a failure has to say why, even in a quiet step
            child.stderr?.pipe(process.stderr);
            child.stdout?.resume();
        }
        return child;
    }

    const prefix = muted(`${opts.tag.padEnd(8)} │ `);
    const relay = (chunk: Buffer, to: NodeJS.WriteStream) => {
        to.write(
            chunk
                .toString()
                .split(/(?<=\n)/)
                .map((line) => `${prefix}${line}`)
                .join("")
        );
    };
    child.stdout?.on("data", (c: Buffer) => relay(c, process.stdout));
    child.stderr?.on("data", (c: Buffer) => relay(c, process.stderr));
    return child;
}

/** run starts a child and rejects unless it exits 0. */
export async function run(command: string, args: string[], opts: Options): Promise<void> {
    const child = start(command, args, opts);
    await new Promise<void>((resolve, reject) => {
        child.once("error", reject);
        child.once("exit", (code, signal) => {
            if (code === 0) resolve();
            else
                reject(
                    new Error(
                        `${command} ${args.join(" ")} exited with ${signal ?? `status ${code}`}`
                    )
                );
        });
    });
}
