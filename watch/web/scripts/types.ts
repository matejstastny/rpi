export type Kind = "anime" | "series" | "movie";

export type Status = "watching" | "ranked" | "queue" | "backlog" | "skipped";

export type Tier = "N" | "S" | "A" | "B" | "C" | "D";

export interface Entry {
    id: string;
    kind: Kind;
    status: Status;
    title: string;
    altTitle?: string;
    tier?: Tier | "";
    rank: number;
    progress?: string;
    predicted?: Tier | "";
    finished?: string;
    note?: string;
    poster?: string;
    year?: number;
    episodes?: number;
    format?: string;
    studio?: string;
    genres?: string[];
    synopsis?: string;
    score?: number;
    provider?: string;
    providerId?: string;
    link?: string;
    added: string;
    updated: string;
}

export interface Settings {
    claudeInstructions: string;
    tierDefinitions: Record<string, string>;
}

export interface Meta {
    kinds: Kind[];
    statuses: Status[];
    tiers: Tier[];
    tmdb: boolean;
}

export interface StatePayload {
    entries: Entry[];
    settings: Settings;
    meta: Meta;
}

export interface SearchResult {
    provider: string;
    providerId: string;
    title: string;
    altTitle?: string;
    year?: number;
    format?: string;
    episodes?: number;
    studio?: string;
    genres?: string[];
    synopsis?: string;
    score?: number;
    poster?: string;
    link?: string;
}

export type NewEntry = Partial<Omit<Entry, "id" | "rank" | "added" | "updated">> & {
    title: string;
    kind: Kind;
};
