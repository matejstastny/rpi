type Attrs = Record<string, string | number | boolean | null | undefined>;

type Child = Node | string | number | null | undefined | false;

export function el<K extends keyof HTMLElementTagNameMap>(
    tag: K,
    attrs: Attrs = {},
    children: Child[] = []
): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);

    for (const [key, value] of Object.entries(attrs)) {
        if (value === null || value === undefined || value === false) continue;
        if (key === "class") node.className = String(value);
        else if (key === "text") node.textContent = String(value);
        else if (key === "html") node.innerHTML = String(value);
        else if (key.startsWith("style:")) node.style.setProperty(key.slice(6), String(value));
        else node.setAttribute(key, value === true ? "" : String(value));
    }

    for (const child of children) {
        if (child === null || child === undefined || child === false) continue;
        node.append(typeof child === "object" ? child : String(child));
    }

    return node;
}

export function need<T extends Element>(selector: string): T {
    const found = document.querySelector<T>(selector);
    if (!found) throw new Error(`missing element: ${selector}`);
    return found;
}

export function clear(node: Element) {
    node.replaceChildren();
}

export function toast(message: string, kind: "ok" | "error" | "info" = "info") {
    const stack = document.getElementById("toasts");
    if (!stack) return;

    const node = el("div", { class: "toast", "data-kind": kind }, [
        el("span", { class: "dot" }),
        message
    ]);
    stack.append(node);

    window.setTimeout(
        () => {
            node.style.transition = "opacity .2s ease";
            node.style.opacity = "0";
            window.setTimeout(() => node.remove(), 220);
        },
        kind === "error" ? 4200 : 2200
    );
}

// debounce keeps the provider search to one request per pause in typing
export function debounce<A extends unknown[]>(fn: (...args: A) => void, wait: number) {
    let timer = 0;
    return (...args: A) => {
        window.clearTimeout(timer);
        timer = window.setTimeout(() => fn(...args), wait);
    };
}
