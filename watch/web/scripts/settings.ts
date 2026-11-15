import { api } from "./api";
import { clear, el, need, toast } from "./dom";
import { TIERS, store, tierColor } from "./state";
import type { Settings } from "./types";

export interface SettingsHooks {
    onSaved: (settings: Settings) => void;
}

const dialog = () => need<HTMLDialogElement>("#settings");

const PLACEHOLDER = `What Claude should know before recommending anything: what you look for,
what breaks immersion, studios and composers you lean towards, what you have
bounced off before.`;

export function openSettings(hooks: SettingsHooks) {
    const body = need<HTMLElement>("#settings-body");
    const foot = need<HTMLElement>("#settings-foot");
    clear(body);
    clear(foot);

    const instructions = el("textarea", {
        rows: 12,
        placeholder: PLACEHOLDER,
        spellcheck: "false"
    }) as HTMLTextAreaElement;
    instructions.value = store.settings.claudeInstructions ?? "";
    instructions.style.minHeight = "210px";

    body.append(
        el("div", { class: "field" }, [
            el("span", { class: "field-label", text: "Instructions for Claude" }),
            instructions,
            el("p", {
                class: "hint",
                text: "Goes at the top of the copied export, above the tier definitions and the library itself."
            })
        ]),
        el("div", { class: "divider" })
    );

    const defs = el("div", { class: "tier-defs" });
    const inputs = new Map<string, HTMLInputElement>();

    for (const tier of TIERS) {
        const input = el("input", { type: "text", placeholder: `What ${tier} means` });
        const field = input as HTMLInputElement;
        field.value = store.settings.tierDefinitions[tier] ?? "";
        inputs.set(tier, field);

        defs.append(
            el("div", { class: "tier-def", "style:--pick": tierColor(tier) }, [
                el("div", { class: "key", text: tier }),
                field
            ])
        );
    }

    body.append(
        el("div", { class: "field" }, [
            el("span", { class: "field-label", text: "Tier definitions" }),
            defs,
            el("p", {
                class: "hint",
                text: "These are not standard S-to-D tiers, so they ship with the export every time."
            })
        ]),
        el("div", { class: "divider" }),
        el("div", { class: "field" }, [
            el("span", { class: "field-label", text: "Hand it to Claude" }),
            el("div", { class: "row-actions" }, [copyButton(), downloadButton()]),
            el("p", {
                class: "hint",
                text: "Copies the instructions, the tier definitions and every list as one markdown paste."
            })
        ])
    );

    const cancel = el("button", { class: "btn btn-ghost", type: "button", text: "Close" });
    cancel.addEventListener("click", () => dialog().close());

    const save = el("button", { class: "btn btn-primary", type: "button", text: "Save settings" });
    save.addEventListener("click", () => {
        const next: Settings = {
            claudeInstructions: instructions.value,
            tierDefinitions: Object.fromEntries(
                [...inputs.entries()].map(([tier, input]) => [tier, input.value])
            )
        };

        save.disabled = true;
        api.saveSettings(next)
            .then((saved) => {
                hooks.onSaved(saved);
                dialog().close();
                toast("Settings saved", "ok");
            })
            .catch((error: unknown) => {
                toast(error instanceof Error ? error.message : "could not save", "error");
            })
            .finally(() => {
                save.disabled = false;
            });
    });

    foot.append(cancel, el("span", { class: "spacer" }), save);

    const node = dialog();
    if (!node.open) node.showModal();
}

function copyButton(): HTMLElement {
    const button = el("button", {
        class: "btn btn-primary",
        type: "button",
        text: "Copy for Claude"
    });

    button.addEventListener("click", () => {
        button.disabled = true;
        api.exportMarkdown("all")
            .then(async (markdown) => {
                await copyText(markdown);
                toast("Whole watchlist copied", "ok");
            })
            .catch((error: unknown) => {
                toast(error instanceof Error ? error.message : "could not copy", "error");
            })
            .finally(() => {
                button.disabled = false;
            });
    });

    return button;
}

function downloadButton(): HTMLElement {
    const link = el("a", {
        class: "btn",
        href: "/api/export?kind=all&download=1",
        download: "watchlist.md",
        text: "Download markdown"
    });
    return link;
}

// clipboard.writeText needs a secure context, which caddy gives us; the
// textarea fallback is for a plain http hit straight at the port
async function copyText(text: string) {
    if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return;
    }

    const scratch = el("textarea", {
        style: "position:fixed;top:-1000px;opacity:0"
    }) as HTMLTextAreaElement;
    scratch.value = text;
    document.body.append(scratch);
    scratch.select();
    document.execCommand("copy");
    scratch.remove();
}
