import { el } from "./dom";
import { tierColor } from "./state";
import type { Entry } from "./types";

// initials are the poster stand-in for anything with no art, so a card never
// renders as an empty grey box
function initials(title: string): string {
    const words = title.split(/\s+/).filter(Boolean).slice(0, 3);
    if (words.length === 0) return "?";
    if (words.length === 1) return words[0]!.slice(0, 2).toUpperCase();
    return words.map((w) => w[0]!.toUpperCase()).join("");
}

export function posterNode(
    entry: Entry,
    flag?: HTMLElement | null,
    showProgress = true
): HTMLElement {
    const wrap = el("div", { class: "poster" });

    if (entry.poster) {
        const img = el("img", {
            src: `/api/art/${entry.id}`,
            alt: "",
            loading: "lazy",
            decoding: "async",
            // chrome starts its own native image drag otherwise, which
            // hijacks the pointer drag the shelves run on
            draggable: "false"
        });
        // the pi caches art on first request; a miss falls back to initials
        img.addEventListener("error", () => {
            img.replaceWith(el("div", { class: "poster-fallback", text: initials(entry.title) }));
        });
        wrap.append(img);
    } else {
        wrap.append(el("div", { class: "poster-fallback", text: initials(entry.title) }));
    }

    if (flag) wrap.append(flag);

    if (showProgress && entry.status === "watching" && entry.progress) {
        wrap.append(el("div", { class: "poster-progress", text: entry.progress }));
    }

    return wrap;
}

export function tierFlag(tier: string): HTMLElement {
    return el("span", {
        class: "poster-flag",
        "data-tier": tier,
        "style:background": tierColor(tier),
        text: tier
    });
}

export function metaLine(entry: Entry): string {
    const bits: string[] = [];
    if (entry.year) bits.push(String(entry.year));
    if (entry.format) bits.push(entry.format);
    else if (entry.episodes) bits.push(`${entry.episodes} ep`);
    return bits.join(" · ");
}

export interface CardOptions {
    flag?: HTMLElement | null;
    onOpen: (entry: Entry) => void;
}

export function cardNode(entry: Entry, opts: CardOptions): HTMLElement {
    const card = el("button", {
        class: "card",
        type: "button",
        "data-id": entry.id,
        title: entry.title
    });

    card.append(
        posterNode(entry, opts.flag),
        el("span", { class: "card-title", text: entry.title })
    );

    const meta = metaLine(entry);
    if (meta) card.append(el("span", { class: "card-meta", text: meta }));

    card.addEventListener("click", () => opts.onOpen(entry));
    return card;
}
