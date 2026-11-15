// the fetch and dom glue every io frontend repeats. nothing here is specific
// to one service: the typed endpoint wrappers stay in each app's own api.ts.

/**
 * request calls one of the Go apis and unwraps its json. A failure response is
 * turned into an Error carrying the api's own `{"error": "..."}` message, which
 * is what every ui shows the user verbatim.
 */
export async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const headers = new Headers(init?.headers);
    if (init?.body && !headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
    }

    const res = await fetch(path, { ...init, headers });
    if (res.status === 204) return undefined as T;

    const text = await res.text();
    let body: unknown = null;
    if (text) {
        try {
            body = JSON.parse(text);
        } catch {
            // the message below covers an unparseable body too
        }
    }

    if (!res.ok) {
        const message =
            body && typeof body === "object" && "error" in body
                ? String((body as { error: unknown }).error)
                : `the server answered ${res.status}`;
        throw new Error(message);
    }

    return body as T;
}

/** need throws instead of returning null, so a renamed id fails at startup. */
export function need<T extends Element>(selector: string): T {
    const found = document.querySelector<T>(selector);
    if (!found) throw new Error(`the page is missing ${selector}`);
    return found;
}

/** bytes matches internal/human.Bytes, so the ui and the logs agree. */
export function bytes(size: number): string {
    if (size <= 0) return "0 B";
    const units = ["B", "kB", "MB", "GB", "TB"];
    const step = Math.min(Math.floor(Math.log10(size) / 3), units.length - 1);
    const value = size / 1000 ** step;
    return `${step === 0 ? value : value.toFixed(1)} ${units[step]}`;
}
