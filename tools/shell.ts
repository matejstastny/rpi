import { execFileSync } from "node:child_process";

/** need exits with a readable message when a required tool is not installed. */
export function need(command: string) {
    try {
        execFileSync("sh", ["-c", `command -v ${command}`], { stdio: "ignore" });
    } catch {
        throw new Error(`${command} is not installed (try: mise install)`);
    }
}

/** bytes matches internal/human.Bytes so build output reads like the logs. */
export function bytes(size: number): string {
    if (size <= 0) return "0 B";
    const units = ["B", "kB", "MB", "GB", "TB"];
    const step = Math.min(Math.floor(Math.log10(size) / 3), units.length - 1);
    const value = size / 1000 ** step;
    return `${step === 0 ? value : value.toFixed(1)} ${units[step]}`;
}
