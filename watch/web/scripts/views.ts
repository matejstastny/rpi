import { cardNode, metaLine, posterNode, tierFlag } from "./cards";
import { clear, el, need } from "./dom";
import { makeDraggable, makeDropZone, type DropHandler } from "./dnd";
import {
    KIND_LABELS,
    TIERS,
    VIEWS,
    VIEW_STATUS,
    bucket,
    countFor,
    store,
    tierColor
} from "./state";
import type { Entry } from "./types";

export interface ViewHooks {
    onOpen: (entry: Entry) => void;
    onDrop: DropHandler;
    onProgress: (entry: Entry, next: number) => void;
    onAdd: () => void;
}

const PROGRESS_RE = /(\d+)/;

export function progressNumber(progress: string | undefined): number {
    const found = progress?.match(PROGRESS_RE);
    return found ? Number(found[1]) : 0;
}

export function renderContent(hooks: ViewHooks) {
    const content = need<HTMLElement>("#content");
    clear(content);

    const meta = VIEWS.find((v) => v.id === store.view)!;
    content.append(
        el("div", { class: "page-head" }, [
            el("h1", { text: `${KIND_LABELS[store.kind]} · ${meta.label}` }),
            el("p", { text: meta.blurb })
        ])
    );

    if (store.view === "tiers") content.append(tierView(hooks));
    else if (store.view === "watching") content.append(watchingView(hooks));
    else content.append(listView(hooks));
}

function tierView(hooks: ViewHooks): HTMLElement {
    const wrap = el("div", { class: "tiers" });
    const total = countFor(store.kind, "tiers");

    for (const tier of TIERS) {
        const entries = bucket("ranked", tier);
        const color = tierColor(tier);

        const head = el(
            "div",
            {
                class: "tier-head",
                "style:--tier-color": color,
                "style:--tier-tint": `color-mix(in oklab, ${color} 7%, transparent)`,
                "style:--tier-tint-strong": `color-mix(in oklab, ${color} 16%, transparent)`
            },
            [
                el("div", { class: "tier-badge" }, [
                    tier,
                    el("span", { class: "tally", text: `${entries.length}` })
                ]),
                el("p", { text: store.settings.tierDefinitions[tier] ?? "" })
            ]
        );

        const shelf = el("div", {
            class: entries.length ? "tier-shelf" : "tier-shelf is-empty",
            "data-empty": total ? "drop something here" : "nothing ranked yet"
        });

        for (const entry of entries) {
            const card = cardNode(entry, { onOpen: hooks.onOpen });
            makeDraggable(card, entry.id);
            shelf.append(card);
        }

        makeDropZone(shelf, { status: "ranked", tier }, hooks.onDrop);
        wrap.append(el("section", { class: "tier" }, [head, shelf]));
    }

    return wrap;
}

function watchingView(hooks: ViewHooks): HTMLElement {
    const entries = bucket("watching");
    if (!entries.length) {
        return emptyState(
            "Nothing on the go",
            "Add something and set it to watching, or move a queued title across.",
            hooks
        );
    }

    const wrap = el("div", { class: "rows" });

    for (const entry of entries) {
        const poster = posterNode(entry, entry.predicted ? tierFlag(entry.predicted) : null, false);
        poster.classList.add("row-poster");
        poster.classList.remove("poster");
        poster.addEventListener("click", () => hooks.onOpen(entry));

        const sub = el("div", { class: "row-sub" });
        const meta = metaLine(entry);
        if (meta) sub.append(el("span", { text: meta }));
        if (entry.studio) sub.append(el("span", { text: entry.studio }));
        if (entry.predicted) {
            sub.append(
                el("span", {
                    class: "chip",
                    "data-tier": entry.predicted,
                    "style:background": tierColor(entry.predicted),
                    text: `predicting ${entry.predicted}`
                })
            );
        }
        if (entry.note) sub.append(el("span", { text: entry.note }));

        const current = progressNumber(entry.progress);
        const value = el("span", { class: "value", text: entry.progress || "no progress" });

        const down = el("button", { type: "button", "aria-label": "Back an episode", text: "−" });
        down.addEventListener("click", () => hooks.onProgress(entry, Math.max(0, current - 1)));

        const up = el("button", { type: "button", "aria-label": "Next episode", text: "+" });
        up.addEventListener("click", () => hooks.onProgress(entry, current + 1));

        const title = el("button", { class: "row-title", type: "button", text: entry.title });
        title.addEventListener("click", () => hooks.onOpen(entry));

        const open = el("button", { class: "btn btn-sm", type: "button", text: "Details" });
        open.addEventListener("click", () => hooks.onOpen(entry));

        wrap.append(
            el("article", { class: "row", "data-id": entry.id }, [
                poster,
                el("div", { class: "row-main" }, [title, sub]),
                el("div", { class: "row-actions" }, [
                    el("div", { class: "stepper" }, [down, value, up]),
                    open
                ])
            ])
        );
    }

    return wrap;
}

function listView(hooks: ViewHooks): HTMLElement {
    const status = VIEW_STATUS[store.view as Exclude<typeof store.view, "tiers">];
    const entries = bucket(status);

    if (!entries.length) {
        const blurbs: Record<string, [string, string]> = {
            queue: ["The queue is empty", "Anything you add lands here by default."],
            backlog: [
                "No backlog",
                "This is the overflow list for when the queue runs dry, keep it long."
            ],
            skipped: [
                "Nothing skipped",
                "Put things here you have decided against, so they stop coming up."
            ]
        };
        const [title, body] = blurbs[status] ?? ["Nothing here", ""];
        return emptyState(title, body, hooks);
    }

    const grid = el("div", { class: "grid" });

    for (const [index, entry] of entries.entries()) {
        const flag =
            status === "queue"
                ? el("span", { class: "poster-flag", text: `#${index + 1}` })
                : entry.predicted
                  ? tierFlag(entry.predicted)
                  : null;

        const card = cardNode(entry, { flag, onOpen: hooks.onOpen });
        makeDraggable(card, entry.id);
        grid.append(card);
    }

    makeDropZone(grid, { status, tier: "" }, hooks.onDrop);
    return grid;
}

function emptyState(title: string, body: string, hooks: ViewHooks): HTMLElement {
    const button = el("button", {
        class: "btn btn-primary",
        type: "button",
        text: "Add something"
    });
    button.addEventListener("click", hooks.onAdd);

    const glyph = el("div", { class: "empty-glyph" });
    glyph.innerHTML =
        '<svg viewBox="0 0 16 16" aria-hidden="true">' +
        '<circle cx="8" cy="8" r="6.6" fill="none" stroke="currentColor" stroke-width="1.2"></circle>' +
        '<path d="M1.4 8a6.6 6.6 0 0 0 13.2 0z" fill="currentColor"></path></svg>';

    return el("div", { class: "empty" }, [
        glyph,
        el("h2", { text: title }),
        el("p", { text: body }),
        button
    ]);
}
