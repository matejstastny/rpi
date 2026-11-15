import { posterNode } from "./cards";
import { clear, el, need, toast } from "./dom";
import { KIND_LABELS, STATUS_LABELS, TIERS, store, tierColor } from "./state";
import type { Entry, Status, Tier } from "./types";

export interface DetailHooks {
    onPatch: (entry: Entry, fields: Record<string, unknown>) => Promise<void>;
    onDelete: (entry: Entry) => Promise<void>;
}

const dialog = () => need<HTMLDialogElement>("#detail");

export function closeDetail() {
    const node = dialog();
    if (node.open) node.close();
}

export function openDetail(entry: Entry, hooks: DetailHooks) {
    const node = dialog();
    render(entry, hooks);
    if (!node.open) node.showModal();
}

// the drawer stays live while the library mutates behind it, so a rerender
// just rebuilds the body from the entry it was handed
export function render(entry: Entry, hooks: DetailHooks) {
    const body = need<HTMLElement>("#detail-body");
    const foot = need<HTMLElement>("#detail-foot");
    clear(body);
    clear(foot);

    need<HTMLElement>("#detail .sheet-head h2").textContent =
        `${KIND_LABELS[entry.kind]} · ${STATUS_LABELS[entry.status]}`;

    const patch = (fields: Record<string, unknown>) => void hooks.onPatch(entry, fields);

    body.append(hero(entry));

    if (entry.synopsis) {
        const text = el("p", { class: "detail-synopsis clamped", text: entry.synopsis });
        const more = el("button", {
            class: "btn btn-ghost btn-sm",
            type: "button",
            text: "Show all"
        });
        more.addEventListener("click", () => {
            const clamped = text.classList.toggle("clamped");
            more.textContent = clamped ? "Show all" : "Show less";
        });
        body.append(text, more, el("div", { class: "divider" }));
    } else {
        body.append(el("div", { class: "divider" }));
    }

    body.append(
        field("List", [
            segmented(
                (["watching", "ranked", "queue", "backlog", "skipped"] as Status[]).map((s) => ({
                    value: s,
                    label: STATUS_LABELS[s]
                })),
                entry.status,
                (value) => patch({ status: value })
            )
        ])
    );

    if (entry.status === "ranked") {
        body.append(
            field("Tier", [
                tierPicker(entry.tier ?? "", false, (value) => patch({ tier: value })),
                el("p", {
                    class: "hint",
                    text: store.settings.tierDefinitions[entry.tier ?? ""] ?? ""
                })
            ])
        );
    }

    if (entry.status !== "ranked") {
        body.append(
            field("Tier prediction", [
                tierPicker(entry.predicted ?? "", true, (value) => patch({ predicted: value })),
                el("p", { class: "hint", text: "Where you think it will land once it is done." })
            ])
        );
    }

    const progress = textInput("text", entry.progress ?? "", "ep 4, or anything you like");
    progress.addEventListener("change", () => patch({ progress: progress.value }));

    const finished = textInput("date", entry.finished ?? "", "");
    finished.addEventListener("change", () => patch({ finished: finished.value }));

    body.append(
        el("div", { class: "field-row" }, [
            field("Progress", [progress]),
            field("Finished on", [finished])
        ])
    );

    const note = el("textarea", {
        rows: 3,
        placeholder: "Anything worth remembering about it"
    }) as HTMLTextAreaElement;
    note.value = entry.note ?? "";
    note.addEventListener("change", () => patch({ note: note.value }));
    body.append(field("Note", [note]));

    const poster = textInput("url", entry.poster ?? "", "https://…");
    poster.addEventListener("change", () => patch({ poster: poster.value }));
    body.append(
        field("Poster url", [
            poster,
            el("p", { class: "hint", text: "Paste a different image if the matched one is wrong." })
        ])
    );

    const remove = el("button", { class: "btn btn-danger", type: "button", text: "Delete" });
    remove.addEventListener("click", () => {
        if (remove.dataset.armed !== "yes") {
            remove.dataset.armed = "yes";
            remove.textContent = "Really delete?";
            window.setTimeout(() => {
                delete remove.dataset.armed;
                remove.textContent = "Delete";
            }, 3500);
            return;
        }
        void hooks.onDelete(entry).then(() => {
            closeDetail();
            toast(`${entry.title} removed`, "ok");
        });
    });

    const done = el("button", { class: "btn", type: "button", text: "Done" });
    done.addEventListener("click", closeDetail);

    foot.append(remove, el("span", { class: "spacer" }), done);
}

function hero(entry: Entry): HTMLElement {
    const chips = el("div", { class: "detail-chips" });
    const add = (text: string) => chips.append(el("span", { class: "chip", text }));

    if (entry.year) add(String(entry.year));
    if (entry.format) add(entry.format);
    if (entry.episodes) add(`${entry.episodes} ep`);
    if (entry.studio) add(entry.studio);
    if (entry.score)
        chips.append(el("span", { class: "chip chip-accent", text: `${entry.score}%` }));
    for (const genre of (entry.genres ?? []).slice(0, 4)) add(genre);

    const column = el("div", {}, [
        el("h2", { text: entry.title }),
        entry.altTitle ? el("p", { class: "detail-alt", text: entry.altTitle }) : null,
        chips,
        entry.link
            ? el("p", { style: "margin:10px 0 0" }, [
                  el("a", {
                      class: "link-out",
                      href: entry.link,
                      target: "_blank",
                      rel: "noreferrer",
                      text: `Open on ${entry.provider === "tmdb" ? "TMDB" : "AniList"} →`
                  })
              ])
            : null
    ]);

    const art = posterNode(entry, null, false);
    return el("div", { class: "detail-hero" }, [art, column]);
}

function field(label: string, children: (Node | null)[]): HTMLElement {
    return el("div", { class: "field" }, [
        el("span", { class: "field-label", text: label }),
        ...children
    ]);
}

function textInput(type: string, value: string, placeholder: string): HTMLInputElement {
    const node = el("input", { type, placeholder }) as HTMLInputElement;
    node.value = value;
    return node;
}

function segmented(
    options: { value: string; label: string }[],
    current: string,
    onPick: (value: string) => void
): HTMLElement {
    const wrap = el("div", { class: "segmented" });
    for (const option of options) {
        const button = el("button", {
            type: "button",
            "aria-pressed": option.value === current ? "true" : "false",
            text: option.label
        });
        button.addEventListener("click", () => {
            if (option.value !== current) onPick(option.value);
        });
        wrap.append(button);
    }
    return wrap;
}

function tierPicker(
    current: string,
    clearable: boolean,
    onPick: (value: Tier | "") => void
): HTMLElement {
    const wrap = el("div", { class: "tier-picker" });

    for (const tier of TIERS) {
        const button = el("button", {
            type: "button",
            "aria-pressed": tier === current ? "true" : "false",
            "style:--pick": tierColor(tier),
            text: tier
        });
        button.addEventListener("click", () => onPick(tier));
        wrap.append(button);
    }

    if (clearable) {
        const none = el("button", {
            type: "button",
            "aria-pressed": current === "" ? "true" : "false",
            "style:--pick": "var(--text-faint)",
            title: "No prediction",
            text: "—"
        });
        none.addEventListener("click", () => onPick(""));
        wrap.append(none);
    }

    return wrap;
}

export { field, segmented, textInput, tierPicker };
