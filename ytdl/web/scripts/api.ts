import { bytes, request } from "@rpi/ui/api";

export { bytes, need } from "@rpi/ui/api";

export interface ProbeResult {
    url: string;
    title: string;
    uploader?: string;
    duration?: string;
    seconds?: number;
    source?: string;
    heights: number[];
}

export interface FileEntry {
    id: string;
    title: string;
    uploader?: string;
    url: string;
    format: string;
    quality?: string;
    name: string;
    size: number;
    duration?: number;
    created: string;
    cached: boolean;
    library: boolean;
    path?: string;
}

export interface JobRequest {
    url: string;
    format: "mp4" | "mp3";
    height: number;
    bitrate: number;
    mode: "cache" | "library" | "both";
    metadata: boolean;
    thumbnail: boolean;
    sponsorblock: boolean;
}

export type Stage = "queued" | "prepare" | "download" | "process" | "done" | "error";

export interface Update {
    stage: Stage;
    label?: string;
    percent: number;
    speed?: number;
    eta?: number;
    total?: number;
    message?: string;
    entry?: FileEntry;
}

export interface FileListing {
    files: FileEntry[];
    cache: { used: number; limit: number };
}

export function probe(url: string, signal?: AbortSignal): Promise<ProbeResult> {
    return request<ProbeResult>("/api/probe", {
        method: "POST",
        body: JSON.stringify({ url }),
        signal
    });
}

export function createJob(job: JobRequest): Promise<{ id: string }> {
    return request<{ id: string }>("/api/jobs", {
        method: "POST",
        body: JSON.stringify(job)
    });
}

export function listFiles(): Promise<FileListing> {
    return request<FileListing>("/api/files");
}

export function dropFile(id: string): Promise<void> {
    return request<void>(`/api/files/${id}`, { method: "DELETE" });
}

export function downloadHref(id: string): string {
    return `/api/files/${id}/download`;
}

export function rate(speed: number): string {
    return speed > 0 ? `${bytes(speed)}/s` : "";
}

export function clock(seconds: number): string {
    if (seconds <= 0) return "";
    const total = Math.round(seconds);
    const minutes = Math.floor(total / 60);
    return minutes > 0 ? `${minutes}m ${total % 60}s` : `${total}s`;
}

export function ago(iso: string): string {
    const seconds = (Date.now() - new Date(iso).getTime()) / 1000;
    if (seconds < 90) return "just now";
    const steps: [number, string][] = [
        [60, "m"],
        [3600, "h"],
        [86400, "d"]
    ];
    if (seconds < 3600) return `${Math.round(seconds / steps[0][0])}m ago`;
    if (seconds < 86400) return `${Math.round(seconds / steps[1][0])}h ago`;
    return `${Math.round(seconds / steps[2][0])}d ago`;
}
