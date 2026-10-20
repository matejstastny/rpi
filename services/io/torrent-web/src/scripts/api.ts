export interface Torrent {
    id: number;
    name: string;
    status: string;
    percent: number;
    size: number;
    downBps: number;
    upBps: number;
    ratio: number;
    peers: number;
    etaSec: number;
    downloaded: number;
    uploaded: number;
    addedAt: number;
    error: string;
}

export interface TorrentListing {
    torrents: Torrent[];
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(path, init);
    if (!response.ok) {
        const body = (await response.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? `the server answered ${response.status}`);
    }
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
}

export function listTorrents(): Promise<TorrentListing> {
    return request<TorrentListing>("/api/torrents");
}

export function addMagnet(magnet: string): Promise<Torrent> {
    return request<Torrent>("/api/torrents", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ magnet })
    });
}

export function addTorrentFile(metainfo: string): Promise<Torrent> {
    return request<Torrent>("/api/torrents", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ metainfo })
    });
}

export function pauseTorrent(id: number): Promise<void> {
    return request<void>(`/api/torrents/${id}/pause`, { method: "POST" });
}

export function resumeTorrent(id: number): Promise<void> {
    return request<void>(`/api/torrents/${id}/resume`, { method: "POST" });
}

export function removeTorrent(id: number, deleteData: boolean): Promise<void> {
    return request<void>(`/api/torrents/${id}?deleteData=${deleteData}`, { method: "DELETE" });
}

export function need<T extends Element>(selector: string): T {
    const found = document.querySelector<T>(selector);
    if (!found) throw new Error(`the page is missing ${selector}`);
    return found;
}

export function bytes(size: number): string {
    if (size <= 0) return "0 B";
    const units = ["B", "kB", "MB", "GB", "TB"];
    const step = Math.min(Math.floor(Math.log10(size) / 3), units.length - 1);
    const value = size / 1000 ** step;
    return `${step === 0 ? value : value.toFixed(1)} ${units[step]}`;
}

export function rate(bps: number): string {
    return bps > 0 ? `${bytes(bps)}/s` : "—";
}

export function duration(seconds: number): string {
    if (seconds <= 0) return "—";
    if (seconds < 60) return `${Math.round(seconds)}s`;
    if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
    if (seconds < 86400) return `${Math.round(seconds / 3600)}h`;
    return `${Math.round(seconds / 86400)}d`;
}

// transmission's own rpc wants a .torrent file as base64, and FileReader is
// the only way to get there without a round trip through a server endpoint
export function fileToBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onerror = () => reject(new Error("could not read that file"));
        reader.onload = () => {
            const result = reader.result as string;
            resolve(result.slice(result.indexOf(",") + 1));
        };
        reader.readAsDataURL(file);
    });
}
