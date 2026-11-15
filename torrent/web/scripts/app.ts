import {
    addMagnet,
    addTorrentFile,
    bytes,
    duration,
    fileToBase64,
    listTorrents,
    need,
    pauseTorrent,
    rate,
    removeTorrent,
    resumeTorrent,
    type Torrent
} from "./api";

const POLL_MS = 3_000;
const DEFAULT_DROP_TEXT = "or drop a .torrent file here";

const magnetForm = need<HTMLFormElement>("#magnet-form");
const magnetField = need<HTMLInputElement>("#magnet");
const dropzone = need<HTMLLabelElement>("#drop");
const dropText = need<HTMLElement>("#drop-text");
const fileInput = need<HTMLInputElement>("#file");
const addNote = need<HTMLElement>("#add-note");

const statDown = need<HTMLElement>("#stat-down");
const statUp = need<HTMLElement>("#stat-up");
const statCount = need<HTMLElement>("#stat-count");
const statActive = need<HTMLElement>("#stat-active");

const list = need<HTMLUListElement>("#torrents");
const empty = need<HTMLElement>("#empty");

function say(message: string, bad = false): void {
    addNote.textContent = message;
    addNote.classList.toggle("bad", bad);
    addNote.classList.toggle("good", !bad);
}

function row(t: Torrent): HTMLLIElement {
    const item = document.createElement("li");

    const title = document.createElement("span");
    title.className = "torrent-title";
    title.textContent = t.name;
    title.title = t.name;

    const bar = document.createElement("div");
    bar.className = "torrent-bar bar";
    const fill = document.createElement("div");
    fill.className = "bar-fill";
    fill.style.width = `${Math.min(t.percent, 100).toFixed(1)}%`;
    bar.append(fill);

    const facts = document.createElement("span");
    facts.className = "torrent-facts";
    const texts = [
        t.downBps + t.upBps > 0 ? `↓${rate(t.downBps)} ↑${rate(t.upBps)}` : bytes(t.size),
        `${t.peers} peers`,
        t.status === "seeding" ? `ratio ${t.ratio.toFixed(2)}` : `eta ${duration(t.etaSec)}`
    ];
    for (const text of [t.error ? "error" : null, `${t.percent.toFixed(1)}%`, ...texts]) {
        if (text === null) continue;
        const tag = document.createElement("span");
        tag.className = "tag";
        tag.textContent = text;
        facts.append(tag);
    }
    const status = document.createElement("span");
    status.className = "tag";
    status.dataset.status = t.error ? "error" : t.status;
    status.textContent = t.error || t.status;
    facts.prepend(status);

    const actions = document.createElement("span");
    actions.className = "torrent-actions";

    const paused = t.status === "paused";
    const toggle = document.createElement("button");
    toggle.className = "ghost";
    toggle.type = "button";
    toggle.textContent = paused ? "resume" : "pause";
    toggle.addEventListener("click", async () => {
        toggle.disabled = true;
        await (paused ? resumeTorrent(t.id) : pauseTorrent(t.id)).catch(() => undefined);
        await refresh();
    });
    actions.append(toggle);

    const drop = document.createElement("button");
    drop.className = "ghost danger";
    drop.type = "button";
    drop.textContent = "remove";
    drop.title = "removes it from transmission, keeps whatever already landed on disk";
    drop.addEventListener("click", async () => {
        drop.disabled = true;
        await removeTorrent(t.id, false).catch(() => undefined);
        await refresh();
    });
    actions.append(drop);

    item.append(title, bar, facts, actions);
    return item;
}

async function refresh(): Promise<void> {
    const listing = await listTorrents().catch(() => null);
    if (!listing) {
        empty.hidden = false;
        empty.textContent = "could not reach the service";
        return;
    }

    const torrents = listing.torrents ?? [];
    let downBps = 0;
    let upBps = 0;
    let active = 0;
    for (const t of torrents) {
        downBps += t.downBps;
        upBps += t.upBps;
        if (t.downBps + t.upBps > 0) active++;
    }
    statDown.textContent = rate(downBps);
    statUp.textContent = rate(upBps);
    statCount.textContent = String(torrents.length);
    statActive.textContent = String(active);

    list.innerHTML = "";
    for (const t of torrents) list.append(row(t));
    empty.hidden = torrents.length > 0;
    empty.textContent = "nothing queued";
}

async function submitMagnet(): Promise<void> {
    const magnet = magnetField.value.trim();
    if (!magnet) return;
    say("");
    try {
        await addMagnet(magnet);
        magnetField.value = "";
        say("added");
        await refresh();
    } catch (error) {
        say(error instanceof Error ? error.message : "could not add that link", true);
    }
}

async function submitFile(file: File): Promise<void> {
    dropText.textContent = file.name;
    try {
        const metainfo = await fileToBase64(file);
        await addTorrentFile(metainfo);
        say(`${file.name} added`);
        await refresh();
    } catch (error) {
        say(error instanceof Error ? error.message : "could not add that file", true);
    } finally {
        dropText.textContent = DEFAULT_DROP_TEXT;
        fileInput.value = "";
    }
}

magnetForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void submitMagnet();
});

fileInput.addEventListener("change", () => {
    const file = fileInput.files?.[0];
    if (file) void submitFile(file);
});

dropzone.addEventListener("dragover", (event) => {
    event.preventDefault();
    dropzone.classList.add("drag");
});
dropzone.addEventListener("dragleave", () => dropzone.classList.remove("drag"));
dropzone.addEventListener("drop", (event) => {
    event.preventDefault();
    dropzone.classList.remove("drag");
    const file = event.dataTransfer?.files?.[0];
    if (file) void submitFile(file);
});

void refresh();
setInterval(() => void refresh(), POLL_MS);

document.addEventListener("visibilitychange", () => {
    if (!document.hidden) void refresh();
});
