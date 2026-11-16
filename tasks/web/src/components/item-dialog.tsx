import { Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { cn } from "@/lib/cn";
import { fromISO, shift, weekdayInitials, weekdayOrder } from "@/lib/dates";
import type { Category, Freq, Item, ItemDraft, Kind, Series } from "@/lib/types";
import { Button, Field, Modal, TimeField, fieldClass } from "./ui";

type Scope = "item" | "series";

const repeats: { value: Freq | "none"; label: string }[] = [
    { value: "none", label: "once" },
    { value: "weekly", label: "weekly" },
    { value: "monthly", label: "monthly" },
    { value: "yearly", label: "yearly" }
];

export function ItemDialog({
    open,
    onClose,
    categories,
    date,
    item,
    series,
    onSubmit,
    onDelete
}: {
    open: boolean;
    onClose: () => void;
    categories: Category[];
    date: string;
    item?: Item | null;
    series?: Series | null;
    onSubmit: (draft: ItemDraft, scope: Scope) => Promise<void>;
    onDelete: (scope: Scope) => Promise<void>;
}) {
    const [scope, setScope] = useState<Scope>("item");
    const [title, setTitle] = useState("");
    const [notes, setNotes] = useState("");
    const [on, setOn] = useState(date);
    const [start, setStart] = useState("");
    const [end, setEnd] = useState("");
    const [category, setCategory] = useState("");
    const [kind, setKind] = useState<Kind>("task");
    const [freq, setFreq] = useState<Freq | "none">("none");
    const [weekdays, setWeekdays] = useState<number[]>([]);
    const [until, setUntil] = useState("");
    const [clocks, setClocks] = useState({ start: true, end: true });
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");

    const editing = Boolean(item);

    useEffect(() => {
        if (!open) return;
        setScope("item");
        setError("");
        setClocks({ start: true, end: true });
        setTitle(item?.title ?? "");
        setNotes(item?.notes ?? "");
        setOn(item?.date ?? date);
        setStart(item?.start ?? "");
        setEnd(item?.end ?? "");
        setCategory(item?.category ?? "");
        setKind(item?.kind ?? "task");
        setFreq(series?.freq ?? "none");
        setWeekdays(series?.weekdays ?? [fromISO(item?.date ?? date).getDay()]);
        setUntil(series?.until ?? shift(item?.date ?? date, 90));
    }, [open, date, item, series]);

    const pick = (next: Scope) => {
        setScope(next);
        setOn(next === "series" ? (series?.from ?? date) : (item?.date ?? date));
    };

    // editing a whole repeat always edits its schedule too
    const showRepeat = (!editing && freq !== "none") || scope === "series";
    const repeating = scope === "series" || (!editing && freq !== "none");

    const draft: ItemDraft = useMemo(
        () => ({
            title,
            notes,
            date: on,
            start,
            end,
            category,
            kind,
            repeat:
                repeating && freq !== "none"
                    ? { freq, weekdays: freq === "weekly" ? weekdays : [], until }
                    : null
        }),
        [title, notes, on, start, end, category, kind, repeating, freq, weekdays, until]
    );

    const submit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!title.trim()) {
            setError("that needs a title");
            return;
        }
        if (!clocks.start || !clocks.end) {
            setError("that is not a time");
            return;
        }
        if (scope === "series" && freq === "none") {
            setError("a repeat needs a schedule");
            return;
        }
        setBusy(true);
        setError("");
        try {
            await onSubmit(draft, scope);
            onClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : "that did not work");
        } finally {
            setBusy(false);
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            title={editing ? "edit" : "new item"}
            description={
                editing
                    ? undefined
                    : "a task and an event are the same thing, give it a time to make it an event"
            }
            footer={
                <>
                    {editing && (
                        <Button
                            variant="danger"
                            type="button"
                            className="mr-auto gap-1.5"
                            onClick={async () => {
                                setBusy(true);
                                try {
                                    await onDelete(scope);
                                    onClose();
                                } finally {
                                    setBusy(false);
                                }
                            }}
                        >
                            <Trash2 className="size-3.5" />
                            {scope === "series" ? "delete the repeat" : "delete"}
                        </Button>
                    )}
                    <Button variant="ghost" type="button" onClick={onClose}>
                        cancel
                    </Button>
                    <Button variant="primary" type="submit" form="item-form" disabled={busy}>
                        {editing ? "save" : "add it"}
                    </Button>
                </>
            }
        >
            <form id="item-form" onSubmit={submit} className="flex flex-col gap-4">
                {item?.series && series && (
                    <div className="flex rounded-xl border border-line bg-base/50 p-1 text-xs">
                        {(
                            [
                                ["item", "just this one"],
                                ["series", "the whole repeat"]
                            ] as const
                        ).map(([value, label]) => (
                            <button
                                key={value}
                                type="button"
                                onClick={() => pick(value)}
                                className={cn(
                                    "flex-1 cursor-pointer rounded-lg px-3 py-1.5 font-medium transition-colors",
                                    scope === value
                                        ? "bg-accent/20 text-accent-soft"
                                        : "text-dim hover:text-bright"
                                )}
                            >
                                {label}
                            </button>
                        ))}
                    </div>
                )}

                <Field label="what">
                    <input
                        autoFocus
                        value={title}
                        onChange={(e) => setTitle(e.target.value)}
                        placeholder="read a chapter, dentist, call mum"
                        className={fieldClass}
                    />
                </Field>

                <Field label="notes" hint="optional">
                    <textarea
                        value={notes}
                        onChange={(e) => setNotes(e.target.value)}
                        rows={2}
                        className={cn(fieldClass, "resize-none")}
                    />
                </Field>

                <Field
                    label="kind"
                    hint={
                        kind === "event"
                            ? "happens or it doesn't: stays on its day, nothing to tick"
                            : "work: follows you forward until it is done or written off"
                    }
                >
                    <div className="flex flex-wrap gap-1.5">
                        <Chip active={kind === "task"} onClick={() => setKind("task")}>
                            task
                        </Chip>
                        <Chip active={kind === "event"} onClick={() => setKind("event")}>
                            event
                        </Chip>
                    </div>
                </Field>

                <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                    <div className="col-span-2">
                        <Field label={repeating ? "starting" : "day"}>
                            <input
                                type="date"
                                value={on}
                                onChange={(e) => setOn(e.target.value)}
                                className={fieldClass}
                            />
                        </Field>
                    </div>
                    <Field label="from" hint="optional">
                        <TimeField
                            value={start}
                            onChange={setStart}
                            onValidity={(ok) => setClocks((c) => ({ ...c, start: ok }))}
                            placeholder="18:00"
                        />
                    </Field>
                    <Field label="to" hint="optional">
                        <TimeField
                            value={end}
                            onChange={setEnd}
                            onValidity={(ok) => setClocks((c) => ({ ...c, end: ok }))}
                            disabled={!start}
                        />
                    </Field>
                </div>

                <Field label="category">
                    <div className="flex flex-wrap gap-1.5">
                        <Chip active={category === ""} onClick={() => setCategory("")}>
                            none
                        </Chip>
                        {categories.map((c) => (
                            <Chip
                                key={c.id}
                                color={c.color}
                                active={category === c.id}
                                onClick={() => setCategory(c.id)}
                            >
                                {c.name}
                            </Chip>
                        ))}
                    </div>
                </Field>

                {!editing && (
                    <Field label="repeat">
                        <div className="flex flex-wrap gap-1.5">
                            {repeats.map((r) => (
                                <Chip
                                    key={r.value}
                                    active={freq === r.value}
                                    onClick={() => setFreq(r.value)}
                                >
                                    {r.label}
                                </Chip>
                            ))}
                        </div>
                    </Field>
                )}

                {showRepeat && freq !== "none" && (
                    <div className="flex flex-col gap-3 rounded-xl border border-line bg-base/40 p-3">
                        {scope === "series" && (
                            <Field label="repeat">
                                <div className="flex flex-wrap gap-1.5">
                                    {repeats
                                        .filter((r) => r.value !== "none")
                                        .map((r) => (
                                            <Chip
                                                key={r.value}
                                                active={freq === r.value}
                                                onClick={() => setFreq(r.value)}
                                            >
                                                {r.label}
                                            </Chip>
                                        ))}
                                </div>
                            </Field>
                        )}

                        {freq === "weekly" && (
                            <Field label="on">
                                <div className="flex gap-1">
                                    {weekdayOrder.map((weekday, n) => (
                                        <button
                                            key={weekday}
                                            type="button"
                                            onClick={() =>
                                                setWeekdays((current) =>
                                                    current.includes(weekday)
                                                        ? current.filter((d) => d !== weekday)
                                                        : [...current, weekday]
                                                )
                                            }
                                            className={cn(
                                                "size-8 cursor-pointer rounded-lg text-xs font-semibold transition-colors",
                                                weekdays.includes(weekday)
                                                    ? "bg-accent text-bright"
                                                    : "bg-overlay/60 text-dim hover:bg-overlay hover:text-bright"
                                            )}
                                        >
                                            {weekdayInitials[n]}
                                        </button>
                                    ))}
                                </div>
                            </Field>
                        )}

                        <Field label="until" hint="a repeat has to end somewhere">
                            <input
                                type="date"
                                value={until}
                                min={on}
                                onChange={(e) => setUntil(e.target.value)}
                                className={fieldClass}
                            />
                        </Field>
                    </div>
                )}

                {error && <p className="text-xs text-red">{error}</p>}
            </form>
        </Modal>
    );
}

function Chip({
    active,
    color,
    onClick,
    children
}: {
    active: boolean;
    color?: string;
    onClick: () => void;
    children: React.ReactNode;
}) {
    return (
        <button
            type="button"
            onClick={onClick}
            style={
                active && color
                    ? {
                          color,
                          background: `color-mix(in oklab, ${color} 18%, transparent)`,
                          borderColor: `color-mix(in oklab, ${color} 45%, transparent)`
                      }
                    : undefined
            }
            className={cn(
                "cursor-pointer rounded-full border px-3 py-1 text-xs font-medium transition-colors",
                active
                    ? "border-accent/50 bg-accent/15 text-accent-soft"
                    : "border-line bg-raise/50 text-dim hover:border-edge hover:text-bright"
            )}
        >
            {children}
        </button>
    );
}
