import { useCallback, useEffect, useRef, useState } from "react";
import { Toaster, toast } from "sonner";
import { api } from "@/lib/api";
import { cheer, ovation } from "@/lib/celebrate";
import { dayNameLong, fromISO, mondayOf, shift } from "@/lib/dates";
import type { Category, Feed, Item, ItemDraft, Status, Week } from "@/lib/types";
import { CategoriesDialog } from "./components/categories-dialog";
import { DayColumn } from "./components/day-column";
import { FeedsDialog } from "./components/feeds-dialog";
import { ItemDialog } from "./components/item-dialog";
import { Button } from "./components/ui";
import { PlanningNudge, WeekHeader } from "./components/week-header";

/** what the dialog is for: a blank item on a day, or an existing one */
type Form = { date: string; item?: Item };

export function App() {
    const [start, setStart] = useState(() => mondayOf(new Date()));
    const [week, setWeek] = useState<Week | null>(null);
    const [failed, setFailed] = useState("");
    const [dragging, setDragging] = useState<string | null>(null);
    const [form, setForm] = useState<Form | null>(null);
    const [categoriesOpen, setCategoriesOpen] = useState(false);
    const [feedsOpen, setFeedsOpen] = useState(false);
    const announced = useRef("");

    useEffect(() => {
        let live = true;
        api.week(start)
            .then((next) => {
                if (!live) return;
                setWeek(next);
                setFailed("");
            })
            .catch((err: Error) => live && setFailed(err.message));
        return () => {
            live = false;
        };
    }, [start]);

    // the sweep happens server side, so the ui owns up to it once per load
    // rather than letting the week quietly rearrange itself
    useEffect(() => {
        if (!week || week.stats.carried === 0) return;
        const key = `${week.today}-${week.stats.carried}`;
        if (announced.current === key) return;
        announced.current = key;
        toast(`${week.stats.carried} carried into today`, {
            description: "still open from before, so they followed you here"
        });
    }, [week]);

    const run = useCallback(async (fn: () => Promise<Week>) => {
        try {
            const next = await fn();
            setWeek(next);
            return next;
        } catch (err) {
            toast.error(err instanceof Error ? err.message : "that did not work");
            throw err;
        }
    }, []);

    const setStatus = useCallback(
        async (item: Item, status: Status) => {
            setWeek((current) =>
                current
                    ? {
                          ...current,
                          days: current.days.map((day) => ({
                              ...day,
                              items: day.items.map((i) => (i.id === item.id ? { ...i, status } : i))
                          }))
                      }
                    : current
            );
            return run(() => api.status(start, item.id, status));
        },
        [run, start]
    );

    const complete = useCallback(
        async (item: Item, rect: DOMRect) => {
            cheer(rect);
            const next = await setStatus(item, "done");

            // finishing the last open thing on a day, or in the week, is
            // worth more than finishing any one item
            const day = next.days.find((d) => d.items.some((i) => i.id === item.id));
            const tasks = day?.items.filter((i) => i.kind !== "event") ?? [];
            if (next.stats.total > 2 && next.stats.open === 0) {
                ovation("week");
                toast.success("the whole week is settled");
            } else if (day && tasks.length > 1 && tasks.every((i) => i.status !== "open")) {
                ovation("day");
                toast.success(`${dayNameLong(day.date)} is clear`);
            }
        },
        [setStatus]
    );

    const give = useCallback(
        async (item: Item) => {
            await setStatus(item, "dropped");
            toast(`${item.title} written off`, { description: "no confetti for that one" });
        },
        [setStatus]
    );

    const move = useCallback(
        async (id: string, date: string) => {
            const item = week?.days.flatMap((d) => d.items).find((i) => i.id === id);
            if (!item || item.date === date) return;
            const draft: ItemDraft = {
                title: item.title,
                notes: item.notes ?? "",
                date,
                start: item.start ?? "",
                end: item.end ?? "",
                category: item.category ?? "",
                kind: item.kind,
                repeat: null
            };
            await run(() => api.edit(start, id, draft));
        },
        [run, start, week]
    );

    const submit = useCallback(
        async (draft: ItemDraft, scope: "item" | "series") => {
            const item = form?.item;
            if (!item) {
                await run(() => api.create(start, draft));
                return;
            }
            if (scope === "series" && item.series) {
                await run(() => api.editSeries(start, item.series!, draft));
                return;
            }
            await run(() => api.edit(start, item.id, draft));
        },
        [form, run, start]
    );

    const remove = useCallback(
        async (scope: "item" | "series") => {
            const item = form?.item;
            if (!item) return;
            if (scope === "series" && item.series) {
                await run(() => api.removeSeries(start, item.series!));
            } else {
                await run(() => api.remove(start, item.id));
            }
            toast(`${item.title} deleted`);
        },
        [form, run, start]
    );

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.metaKey || e.ctrlKey || e.altKey) return;
            if (form || categoriesOpen || feedsOpen) return;
            const active = document.activeElement;
            if (
                active instanceof HTMLElement &&
                (active.isContentEditable ||
                    ["INPUT", "TEXTAREA", "SELECT"].includes(active.tagName))
            ) {
                return;
            }

            switch (e.key) {
                case "n":
                    e.preventDefault();
                    setForm({ date: week?.today ?? start });
                    break;
                case "ArrowLeft":
                    setStart((current) => shift(current, -7));
                    break;
                case "ArrowRight":
                    setStart((current) => shift(current, 7));
                    break;
                case "t":
                    setStart(mondayOf(new Date()));
                    break;
            }
        };
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    }, [form, categoriesOpen, feedsOpen, week, start]);

    const sunday = week ? fromISO(week.today).getDay() === 0 : false;
    const thisWeek = week ? mondayOf(fromISO(week.today)) === week.start : false;

    return (
        <>
            <div className="aurora" />

            <main className="mx-auto flex min-h-dvh w-full max-w-[1760px] flex-col gap-5 px-4 py-6 sm:px-7">
                {failed && !week && (
                    <div className="m-auto flex flex-col items-center gap-3 text-center">
                        <p className="text-sm text-red">{failed}</p>
                        <Button variant="outline" onClick={() => setStart((s) => s)}>
                            try again
                        </Button>
                    </div>
                )}

                {!week && !failed && <Skeleton />}

                {week && (
                    <>
                        <WeekHeader
                            week={week}
                            onPrev={() => setStart(shift(start, -7))}
                            onNext={() => setStart(shift(start, 7))}
                            onToday={() => setStart(mondayOf(new Date()))}
                            onNew={() => setForm({ date: week.today })}
                            onCategories={() => setCategoriesOpen(true)}
                            onFeeds={() => setFeedsOpen(true)}
                        />

                        {sunday && thisWeek && (
                            <PlanningNudge onPlan={() => setStart(shift(start, 7))} />
                        )}

                        <div className="grid flex-1 gap-2.5 md:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7">
                            {week.days.map((day) => (
                                <DayColumn
                                    key={day.date}
                                    day={day}
                                    categories={week.categories}
                                    feeds={week.feeds}
                                    dragging={dragging}
                                    onAdd={(date) => setForm({ date })}
                                    onMove={(id, date) => void move(id, date)}
                                    onComplete={(item, rect) => void complete(item, rect)}
                                    onDrop={(item) => void give(item)}
                                    onReopen={(item) => void setStatus(item, "open")}
                                    onEdit={(item) => setForm({ date: item.date, item })}
                                    onDragStart={setDragging}
                                    onDragEnd={() => setDragging(null)}
                                />
                            ))}
                        </div>
                    </>
                )}
            </main>

            <ItemDialog
                open={Boolean(form)}
                onClose={() => setForm(null)}
                categories={week?.categories ?? []}
                date={form?.date ?? start}
                item={form?.item}
                series={week?.series.find((s) => s.id === form?.item?.series)}
                onSubmit={submit}
                onDelete={remove}
            />

            <CategoriesDialog
                open={categoriesOpen}
                onClose={() => setCategoriesOpen(false)}
                categories={week?.categories ?? []}
                onAdd={async (body: Omit<Category, "id">) => {
                    await run(() => api.addCategory(start, body));
                }}
                onEdit={async (id, body) => {
                    await run(() => api.editCategory(start, id, body));
                }}
                onRemove={async (id) => {
                    await run(() => api.removeCategory(start, id));
                }}
            />

            <FeedsDialog
                open={feedsOpen}
                onClose={() => setFeedsOpen(false)}
                feeds={week?.feeds ?? []}
                onAdd={async (body: Pick<Feed, "name" | "url" | "color" | "kind">) => {
                    await run(() => api.addFeed(start, body));
                }}
                onEdit={async (id, body) => {
                    await run(() => api.editFeed(start, id, body));
                }}
                onSync={async (id) => {
                    await run(() => api.syncFeed(start, id));
                }}
                onRemove={async (id) => {
                    await run(() => api.removeFeed(start, id));
                }}
            />

            <Toaster
                position="bottom-right"
                toastOptions={{
                    style: {
                        background: "var(--color-surface)",
                        border: "1px solid var(--color-line)",
                        color: "var(--color-text)",
                        borderRadius: "14px"
                    }
                }}
            />
        </>
    );
}

function Skeleton() {
    return (
        <div className="flex flex-col gap-5">
            <div className="h-12 w-72 animate-pulse rounded-2xl bg-surface/60" />
            <div className="grid gap-2.5 md:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7">
                {Array.from({ length: 7 }, (_, n) => (
                    <div
                        key={n}
                        className="h-56 animate-pulse rounded-2xl bg-surface/40"
                        style={{ animationDelay: `${n * 60}ms` }}
                    />
                ))}
            </div>
        </div>
    );
}
