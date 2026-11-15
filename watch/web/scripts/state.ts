import { api } from "./api";
import type { Entry, Kind, Meta, Settings, Status, Tier } from "./types";

export type View = "tiers" | "watching" | "queue" | "backlog" | "skipped";

export const TIERS: Tier[] = ["N", "S", "A", "B", "C", "D"];

export const KIND_LABELS: Record<Kind, string> = {
    anime: "Anime",
    series: "Series",
    movie: "Movies"
};

export const STATUS_LABELS: Record<Status, string> = {
    watching: "Watching",
    ranked: "Ranked",
    queue: "Queue",
    backlog: "Backlog",
    skipped: "Not watching"
};

export const VIEWS: { id: View; label: string; blurb: string }[] = [
    { id: "tiers", label: "Tier list", blurb: "Everything you have finished and ranked" },
    { id: "watching", label: "Watching", blurb: "In progress, with a guess at where it lands" },
    { id: "queue", label: "Queue", blurb: "What you are watching next, in order" },
    { id: "backlog", label: "Backlog", blurb: "If there is nothing left in the queue" },
    { id: "skipped", label: "Not watching", blurb: "Deliberately skipped, for whatever reason" }
];

export const VIEW_STATUS: Record<Exclude<View, "tiers">, Status> = {
    watching: "watching",
    queue: "queue",
    backlog: "backlog",
    skipped: "skipped"
};

export function tierColor(tier: string): string {
    return `var(--tier-${tier.toLowerCase()})`;
}

interface Store {
    entries: Entry[];
    settings: Settings;
    meta: Meta;
    kind: Kind;
    view: View;
    filter: string;
}

const listeners = new Set<() => void>();

export const store: Store = {
    entries: [],
    settings: { claudeInstructions: "", tierDefinitions: {} },
    meta: { kinds: ["anime", "series", "movie"], statuses: [], tiers: TIERS, tmdb: false },
    kind: "anime",
    view: "tiers",
    filter: ""
};

export function subscribe(fn: () => void) {
    listeners.add(fn);
}

export function emit() {
    for (const fn of listeners) fn();
}

export async function load() {
    const payload = await api.state();
    store.entries = payload.entries ?? [];
    store.settings = payload.settings;
    store.meta = payload.meta;
    emit();
}

// upsert keeps the local copy in step with what the server just confirmed,
// which is cheaper and less jumpy than refetching the whole library
export function upsert(entry: Entry) {
    const idx = store.entries.findIndex((e) => e.id === entry.id);
    if (idx < 0) store.entries.push(entry);
    else store.entries[idx] = entry;
}

export function forget(id: string) {
    store.entries = store.entries.filter((e) => e.id !== id);
}

export function entryById(id: string): Entry | undefined {
    return store.entries.find((e) => e.id === id);
}

function matchesFilter(entry: Entry): boolean {
    const needle = store.filter.trim().toLowerCase();
    if (!needle) return true;
    return [entry.title, entry.altTitle, entry.studio, entry.note, ...(entry.genres ?? [])]
        .filter(Boolean)
        .some((field) => String(field).toLowerCase().includes(needle));
}

export function inKind(kind: Kind = store.kind): Entry[] {
    return store.entries.filter((e) => e.kind === kind);
}

export function bucket(status: Status, tier: Tier | "" = ""): Entry[] {
    return inKind()
        .filter((e) => e.status === status && (e.tier ?? "") === tier)
        .filter(matchesFilter)
        .sort((a, b) => a.rank - b.rank);
}

export function countFor(kind: Kind, view: View): number {
    const pool = store.entries.filter((e) => e.kind === kind);
    if (view === "tiers") return pool.filter((e) => e.status === "ranked").length;
    return pool.filter((e) => e.status === VIEW_STATUS[view]).length;
}

export function totalFor(kind: Kind): number {
    return store.entries.filter((e) => e.kind === kind).length;
}
