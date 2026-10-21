import { api } from "./api";
import { clear, debounce, el, need, toast } from "./dom";
import { field, segmented, textInput, tierPicker } from "./detail";
import { KIND_LABELS, STATUS_LABELS, store } from "./state";
import type { Entry, Kind, NewEntry, SearchResult, Status, Tier } from "./types";

export interface AddHooks {
    onCreated: (entry: Entry) => void;
}

interface Draft {
    kind: Kind;
    status: Status;
    tier: Tier | "";
    predicted: Tier | "";
    progress: string;
    finished: string;
    note: string;
    picked: SearchResult | null;
    manualTitle: string;
}

let draft: Draft;
let hooks: AddHooks;
let inFlight: AbortController | null = null;
let highlighted = -1;
let lastResults: SearchResult[] = [];

const dialog = () => need<HTMLDialogElement>("#add");

export function openAdd(h: AddHooks) {
    hooks = h;
    draft = {
        kind: store.kind,
        status:
            store.view === "tiers" ? "ranked" : store.view === "watching" ? "watching" : store.view,
        tier: "",
        predicted: "",
        progress: "",
        finished: "",
        note: "",
        picked: null,
        manualTitle: ""
    };
    lastResults = [];
    highlighted = -1;

    render();
    const node = dialog();
    if (!node.open) node.showModal();
    need<HTMLInputElement>("#add-query").focus();
}

function closeAdd() {
    inFlight?.abort();
    inFlight = null;
    const node = dialog();
    if (node.open) node.close();
}

function render() {
    const body = need<HTMLElement>("#add-body");
    const foot = need<HTMLElement>("#add-foot");
    clear(body);
    clear(foot);

    if (draft.picked || draft.manualTitle) {
        body.append(pickedPanel(), ...placementFields());
        foot.append(...pickedFoot());
        return;
    }

    body.append(
        field("Library", [
            segmented(
                (["anime", "series", "movie"] as Kind[]).map((k) => ({
                    value: k,
                    label: KIND_LABELS[k]
                })),
                draft.kind,
                (value) => {
                    draft.kind = value as Kind;
                    render();
                    const input = need<HTMLInputElement>("#add-query");
                    input.focus();
                    if (input.value.trim()) runSearch(input.value);
                }
            )
        ])
    );

    const query = el("input", {
        id: "add-query",
        type: "text",
        placeholder: draft.kind === "anime" ? "Search AniList" : "Search TMDB",
        autocomplete: "off"
    }) as HTMLInputElement;

    const status = el("div", { class: "status-line", id: "add-status" });
    const results = el("div", { class: "results", id: "add-results" });

    const search = debounce((value: string) => runSearch(value), 320);
    query.addEventListener("input", () => {
        highlighted = -1;
        search(query.value);
    });
    query.addEventListener("keydown", (event) => onQueryKeys(event, query));

    body.append(field("Find it", [query]), status, results);

    if (draft.kind !== "anime" && !store.meta.tmdb) {
        setStatus(
            "No TMDB key on the server, so lookup is off for this library. You can still add it by hand.",
            "error"
        );
    } else {
        setStatus("Type a title. Pick the exact match so the poster and metadata come with it.");
    }

    const cancel = el("button", { class: "btn btn-ghost", type: "button", text: "Cancel" });
    cancel.addEventListener("click", closeAdd);

    const manual = el("button", { class: "btn", type: "button", text: "Add by hand" });
    manual.addEventListener("click", () => {
        const typed = need<HTMLInputElement>("#add-query").value.trim();
        draft.manualTitle = typed || "Untitled";
        render();
    });

    foot.append(cancel, el("span", { class: "spacer" }), manual);
}

function onQueryKeys(event: KeyboardEvent, query: HTMLInputElement) {
    if (event.key === "Enter") {
        event.preventDefault();
        if (highlighted >= 0 && lastResults[highlighted]) pick(lastResults[highlighted]!);
        else runSearch(query.value);
        return;
    }

    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    if (!lastResults.length) return;

    event.preventDefault();
    const step = event.key === "ArrowDown" ? 1 : -1;
    highlighted = (highlighted + step + lastResults.length) % lastResults.length;

    const nodes = [...need<HTMLElement>("#add-results").querySelectorAll<HTMLElement>(".result")];
    nodes.forEach((node, index) => {
        node.setAttribute("aria-selected", index === highlighted ? "true" : "false");
        if (index === highlighted) node.scrollIntoView({ block: "nearest" });
    });
}

function setStatus(message: string, kind: "info" | "error" | "busy" = "info") {
    const line = document.getElementById("add-status");
    if (!line) return;
    clear(line);
    line.setAttribute("data-kind", kind === "error" ? "error" : "info");
    if (kind === "busy") line.append(el("span", { class: "spinner" }));
    line.append(message);
}

function runSearch(raw: string) {
    const query = raw.trim();
    const results = document.getElementById("add-results");
    if (!results) return;

    inFlight?.abort();
    lastResults = [];
    highlighted = -1;
    clear(results);

    if (query.length < 2) {
        setStatus("Type a title. Pick the exact match so the poster and metadata come with it.");
        return;
    }

    setStatus("Searching…", "busy");
    const controller = new AbortController();
    inFlight = controller;

    api.search(draft.kind, query, controller.signal)
        .then((payload) => {
            if (controller.signal.aborted) return;
            lastResults = payload.results ?? [];

            if (!lastResults.length) {
                setStatus(`Nothing came back for “${query}”. Add it by hand instead.`);
                return;
            }

            setStatus(`${lastResults.length} matches · ↑↓ to move, Enter to pick`);
            for (const result of lastResults) results.append(resultNode(result));
        })
        .catch((error: unknown) => {
            if (controller.signal.aborted) return;
            setStatus(error instanceof Error ? error.message : "search failed", "error");
        });
}

function resultNode(result: SearchResult): HTMLElement {
    const node = el("button", { class: "result", type: "button", "aria-selected": "false" }, [
        result.poster
            ? el("img", { src: previewUrl(result.poster), alt: "", loading: "lazy" })
            : el("div", { class: "result-blank" }),
        el("div", {}, [
            el("div", { class: "result-title", text: result.title }),
            el("div", { class: "result-sub", text: resultSub(result) })
        ])
    ]);

    node.addEventListener("click", () => pick(result));
    return node;
}

// thumbnails come back through the pi, so the browser never fetches from
// anilist or tmdb itself
function previewUrl(poster: string): string {
    return `/api/preview?url=${encodeURIComponent(poster)}`;
}

function resultSub(result: SearchResult): string {
    return [
        result.year ? String(result.year) : null,
        result.format,
        result.episodes ? `${result.episodes} ep` : null,
        result.studio,
        result.altTitle
    ]
        .filter(Boolean)
        .join(" · ");
}

function pick(result: SearchResult) {
    draft.picked = result;
    draft.manualTitle = "";
    render();
}

function pickedPanel(): HTMLElement {
    const picked = draft.picked;

    const art = picked?.poster
        ? el("img", { src: previewUrl(picked.poster), alt: "" })
        : el("div", { class: "result-blank" });

    const title = picked?.title ?? draft.manualTitle;

    const column = el("div", {}, [
        el("h3", { text: title }),
        el("div", { class: "result-sub", text: picked ? resultSub(picked) : "Added by hand" }),
        picked?.synopsis ? el("p", { class: "synopsis", text: picked.synopsis }) : null
    ]);

    if (!picked) {
        const input = textInput("text", draft.manualTitle, "Title");
        input.addEventListener("input", () => {
            draft.manualTitle = input.value;
        });
        clear(column);
        column.append(
            el("span", { class: "field-label", text: "Title" }),
            input,
            el("p", {
                class: "hint",
                text: "No provider match, so there is no poster for this one."
            })
        );
    }

    const back = el("button", {
        class: "btn btn-ghost btn-sm",
        type: "button",
        text: "← Pick again"
    });
    back.addEventListener("click", () => {
        draft.picked = null;
        draft.manualTitle = "";
        render();
    });

    return el("div", {}, [el("div", { class: "picked" }, [art, column]), back]);
}

function placementFields(): HTMLElement[] {
    const out: HTMLElement[] = [el("div", { class: "divider" })];

    out.push(
        field("Goes into", [
            segmented(
                (["ranked", "watching", "queue", "backlog", "skipped"] as Status[]).map((s) => ({
                    value: s,
                    label: STATUS_LABELS[s]
                })),
                draft.status,
                (value) => {
                    draft.status = value as Status;
                    render();
                }
            )
        ])
    );

    if (draft.status === "ranked") {
        out.push(
            field("Tier", [
                tierPicker(draft.tier, false, (value) => {
                    draft.tier = value;
                    render();
                }),
                el("p", {
                    class: "hint",
                    text: draft.tier
                        ? (store.settings.tierDefinitions[draft.tier] ?? "")
                        : "Pick one."
                })
            ])
        );

        const finished = textInput("date", draft.finished, "");
        finished.addEventListener("change", () => {
            draft.finished = finished.value;
        });
        out.push(field("Finished on", [finished]));
    } else {
        out.push(
            field("Tier prediction", [
                tierPicker(draft.predicted, true, (value) => {
                    draft.predicted = value;
                    render();
                })
            ])
        );
    }

    if (draft.status === "watching") {
        const progress = textInput("text", draft.progress, "ep 1");
        progress.addEventListener("input", () => {
            draft.progress = progress.value;
        });
        out.push(field("Progress", [progress]));
    }

    const note = el("textarea", { rows: 2, placeholder: "Optional note" }) as HTMLTextAreaElement;
    note.value = draft.note;
    note.addEventListener("input", () => {
        draft.note = note.value;
    });
    out.push(field("Note", [note]));

    return out;
}

function pickedFoot(): HTMLElement[] {
    const cancel = el("button", { class: "btn btn-ghost", type: "button", text: "Cancel" });
    cancel.addEventListener("click", closeAdd);

    const save = el("button", {
        class: "btn btn-primary",
        type: "button",
        text: `Add to ${STATUS_LABELS[draft.status].toLowerCase()}`
    });
    save.addEventListener("click", () => void commit(save));

    return [cancel, el("span", { class: "spacer" }), save];
}

async function commit(button: HTMLButtonElement) {
    const picked = draft.picked;
    const title = picked?.title ?? draft.manualTitle.trim();

    if (!title) {
        toast("That needs a title", "error");
        return;
    }
    if (draft.status === "ranked" && !draft.tier) {
        toast("Pick a tier first", "error");
        return;
    }

    const payload: NewEntry = {
        kind: draft.kind,
        status: draft.status,
        title,
        altTitle: picked?.altTitle,
        tier: draft.status === "ranked" ? draft.tier : "",
        predicted: draft.status === "ranked" ? "" : draft.predicted,
        progress: draft.progress,
        finished: draft.finished,
        note: draft.note,
        poster: picked?.poster,
        year: picked?.year,
        episodes: picked?.episodes,
        format: picked?.format,
        studio: picked?.studio,
        genres: picked?.genres,
        synopsis: picked?.synopsis,
        score: picked?.score,
        provider: picked?.provider ?? "manual",
        providerId: picked?.providerId,
        link: picked?.link
    };

    button.disabled = true;
    try {
        const created = await api.create(payload);
        hooks.onCreated(created);
        closeAdd();
        toast(`${created.title} added`, "ok");
    } catch (error) {
        toast(error instanceof Error ? error.message : "could not add that", "error");
        button.disabled = false;
    }
}
