const colour = process.stdout.isTTY && !process.env.NO_COLOR;
const paint = (code: string, value: string) =>
    colour ? `\u001B[${code}m${value}\u001B[0m` : value;

export const bold = (v: string) => paint("1", v);
export const muted = (v: string) => paint("90", v);
export const green = (v: string) => paint("32", v);
export const cyan = (v: string) => paint("36", v);
export const yellow = (v: string) => paint("33", v);
export const red = (v: string) => paint("31", v);

export const step = (v: string) => console.log(`${cyan("▸")} ${v}`);
export const done = (v: string) => console.log(`${green("✓")} ${v}`);

export function die(err: unknown): never {
    const message = err instanceof Error ? err.message : String(err);
    console.error(`\n${red("error:")} ${message}`);
    process.exit(1);
}
