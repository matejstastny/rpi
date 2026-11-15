import { el } from "./dom";
import type { Status, Tier } from "./types";

export type DropHandler = (id: string, status: Status, tier: Tier | "", index: number) => void;

interface Zone {
    node: HTMLElement;
    status: Status;
    tier: Tier | "";
    onDrop: DropHandler;
}

interface Drag {
    id: string;
    card: HTMLElement;
    ghost: HTMLElement;
    offsetX: number;
    offsetY: number;
    zone: Zone | null;
    index: number;
}

// pointer events rather than the html5 drag api: that one cannot be driven
// from touch at all, gives no control over the drag image, and fires nothing
// useful while the pointer is between two shelves
const zones = new WeakMap<HTMLElement, Zone>();

let pending: { id: string; card: HTMLElement; x: number; y: number } | null = null;
let drag: Drag | null = null;
let marker: HTMLElement | null = null;
let suppressClick = false;

const START_DISTANCE = 5;
const EDGE_BAND = 90;
const EDGE_SPEED = 14;

export function makeDraggable(node: HTMLElement, id: string) {
    node.style.touchAction = "none";

    node.addEventListener("pointerdown", (event) => {
        if (event.button !== 0) return;
        pending = { id, card: node, x: event.clientX, y: event.clientY };
    });

    // chrome will still start a native drag off the card's own content, and
    // once it does no pointermove or pointerup arrives here at all
    node.addEventListener("dragstart", (event) => event.preventDefault());

    // a drag ends on the card that started it, so the click it would
    // otherwise fire has to be swallowed exactly once
    node.addEventListener(
        "click",
        (event) => {
            if (!suppressClick) return;
            suppressClick = false;
            event.preventDefault();
            event.stopPropagation();
        },
        true
    );
}

export function makeDropZone(
    node: HTMLElement,
    target: { status: Status; tier: Tier | "" },
    onDrop: DropHandler
) {
    zones.set(node, { node, status: target.status, tier: target.tier, onDrop });
    node.dataset.zone = "";
}

function zoneAt(x: number, y: number): Zone | null {
    for (const node of document.elementsFromPoint(x, y)) {
        if (!(node instanceof HTMLElement)) continue;
        const found = node.closest<HTMLElement>("[data-zone]");
        if (found) return zones.get(found) ?? null;
    }
    return null;
}

// index is worked out from the pointer against each card's own box, so a
// wrapped shelf still inserts where the gap is showing
function insertionIndex(zone: Zone, x: number, y: number): number {
    const cards = cardsIn(zone);
    for (let i = 0; i < cards.length; i += 1) {
        const box = cards[i]!.getBoundingClientRect();
        if (y < box.bottom && x < box.left + box.width / 2) return i;
    }
    return cards.length;
}

function cardsIn(zone: Zone): HTMLElement[] {
    return [...zone.node.querySelectorAll<HTMLElement>(".card")].filter((c) => c !== drag?.card);
}

function showMarker(zone: Zone, index: number) {
    if (!marker) marker = el("div", { class: "drop-marker" });
    const before = cardsIn(zone)[index];
    if (before) zone.node.insertBefore(marker, before);
    else zone.node.append(marker);
}

function clearMarker() {
    marker?.remove();
    marker = null;
    for (const node of document.querySelectorAll("[data-drop]")) {
        node.removeAttribute("data-drop");
    }
}

function begin(event: PointerEvent) {
    if (!pending) return;

    const box = pending.card.getBoundingClientRect();
    const poster = pending.card.querySelector(".poster");

    const ghost = el("div", { class: "drag-ghost" });
    ghost.style.width = `${box.width}px`;
    ghost.append(poster ? poster.cloneNode(true) : pending.card.cloneNode(true));
    document.body.append(ghost);

    drag = {
        id: pending.id,
        card: pending.card,
        ghost,
        offsetX: pending.x - box.left,
        offsetY: pending.y - box.top,
        zone: null,
        index: 0
    };

    pending.card.classList.add("dragging");
    document.body.classList.add("is-dragging");
    pending.card.setPointerCapture(event.pointerId);
    pending = null;
}

function moveGhost(x: number, y: number) {
    if (!drag) return;
    drag.ghost.style.transform = `translate(${x - drag.offsetX}px, ${y - drag.offsetY}px)`;
}

// dragging a card to a tier that is off-screen has to be possible, so the
// scroller creeps when the pointer sits near its top or bottom edge
function edgeScroll(y: number) {
    const content = document.querySelector<HTMLElement>(".content");
    if (!content) return;

    const box = content.getBoundingClientRect();
    if (y < box.top + EDGE_BAND) content.scrollTop -= EDGE_SPEED;
    else if (y > box.bottom - EDGE_BAND) content.scrollTop += EDGE_SPEED;
}

function onMove(event: PointerEvent) {
    if (pending) {
        const far =
            Math.abs(event.clientX - pending.x) > START_DISTANCE ||
            Math.abs(event.clientY - pending.y) > START_DISTANCE;
        if (!far) return;
        begin(event);
    }

    if (!drag) return;
    event.preventDefault();

    moveGhost(event.clientX, event.clientY);
    edgeScroll(event.clientY);

    const zone = zoneAt(event.clientX, event.clientY);
    if (!zone) {
        drag.zone = null;
        clearMarker();
        return;
    }

    clearMarker();
    (zone.node.closest(".tier") ?? zone.node).setAttribute("data-drop", "active");

    drag.zone = zone;
    drag.index = insertionIndex(zone, event.clientX, event.clientY);
    showMarker(zone, drag.index);
}

function onUp(event: PointerEvent) {
    pending = null;
    if (!drag) return;

    const { id, zone, index, card, ghost } = drag;

    ghost.remove();
    card.classList.remove("dragging");
    document.body.classList.remove("is-dragging");
    if (card.hasPointerCapture(event.pointerId)) card.releasePointerCapture(event.pointerId);
    clearMarker();
    drag = null;
    suppressClick = true;

    // a drag that ends outside every shelf is a cancel, not a move
    if (zone) zone.onDrop(id, zone.status, zone.tier, index);
    else window.setTimeout(() => (suppressClick = false), 0);
}

document.addEventListener("pointermove", onMove, { passive: false });
document.addEventListener("pointerup", onUp);
document.addEventListener("pointercancel", onUp);
