// `mise run apps`: the manifest as a table, so you never have to guess which
// port or domain belongs to what.
import { apps, hosts } from "./manifest.ts";
import { bold, cyan, die, muted } from "./term.ts";

function main() {
    const all = apps();
    const pad = Math.max(...all.map((a) => a.name.length));

    for (const h of hosts()) {
        console.log(`\n${bold(`${h.glyph} ${h.name}`)} ${muted(`${h.plane} plane · ${h.blurb}`)}`);
        const mine = all.filter((a) => a.host === h.name);
        if (!mine.length) {
            console.log(muted("  no apps"));
            continue;
        }
        for (const a of mine) {
            console.log(
                `  ${a.name.padEnd(pad)}  ${muted("api")} ${cyan(String(a.api))}` +
                    `  ${muted("dev")} ${cyan(String(a.web))}  ${muted(a.domain)}`
            );
        }
    }

    console.log(`\n${muted("mise run dev <app>")}    one app locally`);
    console.log(`${muted("mise run build <app>")}  cross-compile for the pis`);
    console.log(`${muted("mise run deploy <host>")} build, push and install`);
}

try {
    main();
} catch (err) {
    die(err);
}
