import { api } from "./api";
import { openAdd } from "./add";
import { clear, el, need, toast } from "./dom";
import { closeDetail, openDetail, render as renderDetail } from "./detail";
import { openSettings } from "./settings";
import {
    KIND_LABELS,
    TIERS,
    VIEWS,
    countFor,
    emit,
    entryById,
    forget,
    load,
    store,
    subscribe,
    tierColor,
    totalFor,
    upsert,
    type View
} from "./state";
import { renderContent } from "./views";
import type { Entry, Kind, Status, Tier } from "./types";

let openEntryId: string | null = null;

const hooks = {
    onOpen(entry: Entry) {
        openEntryId = entry.id;
        openDetail(entry, detailHooks);
    },
    onDrop(id: string, status: Status, tier: Tier | "", index: number) {
        void api
            .move(id, status, tier, index)
            .then((updated) => {
                upsert(updated);
                emit();
                // the server renumbers the whole bucket around the drop, so
                // the local ranks of its neighbours are now one behind
                void reconcile();
            })
            .catch((error: unknown) => {
                toast(error instanceof Error ? error.message : "could not move that", "error");
                void reconcile();
            });
    },
    onProgress(entry: Entry, next: number) {
        void patch(entry, { progress: next > 0 ? `ep ${next}` : "" });
    },
    onAdd() {
        openAdd({
            onCreated(entry) {
                upsert(entry);
                // land on the view the new entry went into, so it is visible
                store.kind = entry.kind;
                store.view = entry.status === "ranked" ? "tiers" : (entry.status as View);
                emit();
                void reconcile();
            }
        });
    }
};

const detailHooks = {
    async onPatch(entry: Entry, fields: Record<string, unknown>) {
        await patch(entry, fields);
    },
    async onDelete(entry: Entry) {
        try {
            await api.remove(entry.id);
            forget(entry.id);
            openEntryId = null;
            emit();
            await reconcile();
        } catch (error) {
            toast(error instanceof Error ? error.message : "could not delete that", "error");
        }
    }
};

async function patch(entry: Entry, fields: Record<string, unknown>) {
    try {
        const updated = await api.patch(entry.id, fields);
        const moved =
            updated.status !== entry.status || (updated.tier ?? "") !== (entry.tier ?? "");
        upsert(updated);
        emit();
        if (moved) await reconcile();
    } catch (error) {
        toast(error instanceof Error ? error.message : "could not save that", "error");
    }
}

// a reload after anything that renumbers a bucket. the optimistic upsert has
// already repainted, so this only ever corrects the neighbours' ranks.
async function reconcile() {
    try {
        await load();
    } catch {
        // a failed refresh leaves the optimistic view in place, which is
        // still closer to the truth than throwing it away
    }
}

function renderKinds() {
    const nav = need<HTMLElement>("#kinds");
    clear(nav);

    for (const kind of ["anime", "series", "movie"] as Kind[]) {
        const button = el(
            "button",
            {
                class: "kind",
                type: "button",
                role: "tab",
                "aria-selected": kind === store.kind ? "true" : "false"
            },
            [KIND_LABELS[kind], el("span", { class: "tally", text: `${totalFor(kind)}` })]
        );
        button.addEventListener("click", () => {
            store.kind = kind;
            emit();
        });
        nav.append(button);
    }
}

function renderViews() {
    const nav = need<HTMLElement>("#views");
    clear(nav);

    for (const view of VIEWS) {
        const button = el(
            "button",
            {
                class: "view",
                type: "button",
                title: view.blurb,
                "aria-selected": view.id === store.view ? "true" : "false"
            },
            [
                el("span", { class: "view-dot" }),
                el("span", { text: view.label }),
                el("span", { class: "tally", text: `${countFor(store.kind, view.id)}` })
            ]
        );
        button.addEventListener("click", () => {
            store.view = view.id;
            emit();
        });
        nav.append(button);
    }
}

function renderLegend() {
    const box = need<HTMLElement>("#legend");
    clear(box);

    for (const tier of TIERS) {
        const text = store.settings.tierDefinitions[tier];
        if (!text) continue;
        box.append(
            el("div", { class: "legend-row" }, [
                el("span", {
                    class: "chip",
                    "data-tier": tier,
                    "style:background": tierColor(tier),
                    text: tier
                }),
                el("p", { text })
            ])
        );
    }
}

function renderAll() {
    renderKinds();
    renderViews();
    renderLegend();
    renderContent(hooks);

    // the drawer is live: a patch rerenders it in place rather than closing it
    if (openEntryId) {
        const entry = entryById(openEntryId);
        if (entry) renderDetail(entry, detailHooks);
        else closeDetail();
    }
}

function wireChrome() {
    need<HTMLElement>("#add-open").addEventListener("click", hooks.onAdd);
    need<HTMLElement>("#settings-open").addEventListener("click", () =>
        openSettings({
            onSaved(settings) {
                store.settings = settings;
                emit();
            }
        })
    );

    const filter = need<HTMLInputElement>("#filter");
    filter.addEventListener("input", () => {
        store.filter = filter.value;
        renderContent(hooks);
    });

    for (const button of document.querySelectorAll<HTMLElement>("[data-close]")) {
        button.addEventListener("click", () => {
            const target = document.getElementById(button.dataset.close ?? "");
            if (target instanceof HTMLDialogElement) target.close();
        });
    }

    need<HTMLDialogElement>("#detail").addEventListener("close", () => {
        openEntryId = null;
    });

    document.addEventListener("keydown", (event) => {
        if (event.metaKey || event.ctrlKey || event.altKey) return;

        const active = document.activeElement;
        const typing =
            active instanceof HTMLInputElement ||
            active instanceof HTMLTextAreaElement ||
            active instanceof HTMLSelectElement;
        if (typing) return;
        if (document.querySelector("dialog[open]")) return;

        if (event.key === "/") {
            event.preventDefault();
            filter.focus();
            filter.select();
            return;
        }
        if (event.key === "a") {
            event.preventDefault();
            hooks.onAdd();
            return;
        }

        const kindKeys: Record<string, Kind> = { "1": "anime", "2": "series", "3": "movie" };
        if (kindKeys[event.key]) {
            store.kind = kindKeys[event.key]!;
            emit();
        }
    });
}

async function boot() {
    subscribe(renderAll);
    wireChrome();

    try {
        await load();
    } catch (error) {
        toast(error instanceof Error ? error.message : "could not load the watchlist", "error");
        renderAll();
    }
}

void boot();
