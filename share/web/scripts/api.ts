import { request } from "@rpi/ui/api";

export { bytes, need } from "@rpi/ui/api";

export interface FileEntry {
    id: string;
    name: string;
    size: number;
    created: string;
}

export interface FileListing {
    files: FileEntry[];
    limit: number;
}

export function listFiles(): Promise<FileListing> {
    return request<FileListing>("/api/files");
}

export function dropFile(id: string): Promise<void> {
    return request<void>(`/api/files/${id}`, { method: "DELETE" });
}

export function downloadHref(file: FileEntry): string {
    return `/dl/${file.id}/${encodeURIComponent(file.name)}`;
}

export function ago(iso: string): string {
    const seconds = (Date.now() - new Date(iso).getTime()) / 1000;
    if (seconds < 90) return "just now";
    if (seconds < 3600) return `${Math.round(seconds / 60)}m ago`;
    if (seconds < 86400) return `${Math.round(seconds / 3600)}h ago`;
    return `${Math.round(seconds / 86400)}d ago`;
}

// upload goes through XMLHttpRequest instead of fetch, the only way to get
// upload progress events for a large file
export function upload(file: File, onProgress: (fraction: number) => void): Promise<FileEntry> {
    return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open("POST", "/api/files");
        xhr.upload.onprogress = (event) => {
            if (event.lengthComputable) onProgress(event.loaded / event.total);
        };
        xhr.onerror = () => reject(new Error("upload did not finish"));
        xhr.onload = () => {
            let body: { error?: string } & Partial<FileEntry> = {};
            try {
                body = JSON.parse(xhr.responseText);
            } catch {
                // the error message below covers an unparseable body too
            }
            if (xhr.status >= 200 && xhr.status < 300 && body.id) {
                resolve(body as FileEntry);
            } else {
                reject(new Error(body.error ?? `the server answered ${xhr.status}`));
            }
        };
        const form = new FormData();
        form.append("file", file, file.name);
        xhr.send(form);
    });
}
