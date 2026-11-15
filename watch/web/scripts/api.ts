import type { Entry, NewEntry, SearchResult, Settings, StatePayload, Status, Tier } from "./types";
import { request } from "@rpi/ui/api";

export const api = {
    state: () => request<StatePayload>("/api/state"),

    create: (entry: NewEntry) =>
        request<Entry>("/api/entries", { method: "POST", body: JSON.stringify(entry) }),

    patch: (id: string, fields: Record<string, unknown>) =>
        request<Entry>(`/api/entries/${id}`, { method: "PATCH", body: JSON.stringify(fields) }),

    move: (id: string, status: Status, tier: Tier | "", index: number) =>
        request<Entry>(`/api/entries/${id}/move`, {
            method: "POST",
            body: JSON.stringify({ status, tier, index })
        }),

    remove: (id: string) => request<void>(`/api/entries/${id}`, { method: "DELETE" }),

    saveSettings: (settings: Settings) =>
        request<Settings>("/api/settings", { method: "PUT", body: JSON.stringify(settings) }),

    search: (kind: string, q: string, signal?: AbortSignal) =>
        request<{ results: SearchResult[] }>(
            `/api/search?kind=${encodeURIComponent(kind)}&q=${encodeURIComponent(q)}`,
            { signal }
        ),

    exportMarkdown: async (kind: string) => {
        const res = await fetch(`/api/export?kind=${encodeURIComponent(kind)}`);
        if (!res.ok) throw new Error(`export failed (${res.status})`);
        return res.text();
    }
};
