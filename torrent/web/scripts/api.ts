import { bytes, request } from "@rpi/ui/api";

export { bytes, need } from "@rpi/ui/api";

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
