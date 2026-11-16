import type { Category, Feed, ItemDraft, Status, Week } from "./types";

/** every call answers with the whole week the ui is looking at, so there is
 *  one source of truth and nothing to merge by hand on this side */
async function call(path: string, init?: RequestInit): Promise<Week> {
    const res = await fetch(path, {
        ...init,
        headers: init?.body ? { "Content-Type": "application/json" } : undefined
    });

    if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? `that did not work (${res.status})`);
    }
    return (await res.json()) as Week;
}

const send = (path: string, week: string, method: string, body?: unknown) =>
    call(`${path}${path.includes("?") ? "&" : "?"}week=${week}`, {
        method,
        body: body === undefined ? undefined : JSON.stringify(body)
    });

export const api = {
    week: (week?: string) => call(`/api/week${week ? `?week=${week}` : ""}`),

    create: (week: string, draft: ItemDraft) => send("/api/items", week, "POST", draft),
    edit: (week: string, id: string, draft: ItemDraft) =>
        send(`/api/items/${id}`, week, "PATCH", draft),
    status: (week: string, id: string, status: Status) =>
        send(`/api/items/${id}/status`, week, "POST", { status }),
    remove: (week: string, id: string) => send(`/api/items/${id}`, week, "DELETE"),

    editSeries: (week: string, id: string, draft: ItemDraft) =>
        send(`/api/series/${id}`, week, "PATCH", draft),
    removeSeries: (week: string, id: string) => send(`/api/series/${id}`, week, "DELETE"),

    addCategory: (week: string, body: Omit<Category, "id">) =>
        send("/api/categories", week, "POST", body),
    editCategory: (week: string, id: string, body: Omit<Category, "id">) =>
        send(`/api/categories/${id}`, week, "PATCH", body),
    removeCategory: (week: string, id: string) => send(`/api/categories/${id}`, week, "DELETE"),

    addFeed: (week: string, body: Pick<Feed, "name" | "url" | "color" | "kind">) =>
        send("/api/feeds", week, "POST", body),
    editFeed: (week: string, id: string, body: Pick<Feed, "name" | "color" | "kind">) =>
        send(`/api/feeds/${id}`, week, "PATCH", body),
    removeFeed: (week: string, id: string) => send(`/api/feeds/${id}`, week, "DELETE"),
    syncFeed: (week: string, id: string) => send(`/api/feeds/${id}/sync`, week, "POST")
};
